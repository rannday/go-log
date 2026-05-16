package httpx

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rannday/go-log"
)

func captureMiddleware(t *testing.T, fn func()) string {
	t.Helper()

	logx.Reset()

	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug, AddSource: false})
	logx.SetLogger(slog.New(handler))

	fn()

	return buf.String()
}

func TestMiddleware_LogsStatus(t *testing.T) {
	out := captureMiddleware(t, func() {
		handler := HTTPMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(404)
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
	})

	if !strings.Contains(out, "status=404") {
		t.Fatalf("expected status 404 log")
	}
}

func TestMiddleware_RecoversPanic(t *testing.T) {
	out := captureMiddleware(t, func() {
		handler := HTTPMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("boom")
		}))

		req := httptest.NewRequest("GET", "/panic", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
	})

	if !strings.Contains(out, "http handler panic") {
		t.Fatalf("expected panic log")
	}

	if !strings.Contains(out, "stack") {
		t.Fatalf("expected stack trace")
	}
}
