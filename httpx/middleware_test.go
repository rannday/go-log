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
	defer logx.Reset()

	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug, AddSource: false})
	logx.SetLogger(slog.New(handler))

	fn()

	return buf.String()
}

func TestMiddleware_LogsStatusAndBytes(t *testing.T) {
	out := captureMiddleware(t, func() {
		handler := HTTPMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(404)
			_, _ = w.Write([]byte("hey"))
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
	})

	if !strings.Contains(out, "status=404") {
		t.Fatalf("expected status 404 log, got: %s", out)
	}
	if !strings.Contains(out, "bytes=3") {
		t.Fatalf("expected byte count in log, got: %s", out)
	}
}

func TestMiddleware_PropagatesRequestIDFromHeader(t *testing.T) {
	var got string
	handler := HTTPMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := logx.RequestID(r.Context())
		if !ok {
			t.Fatalf("expected request id in context")
		}
		got = id
	}))

	req := httptest.NewRequest("GET", "/header", nil)
	req.Header.Set("X-Request-ID", "rid-header")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if got != "rid-header" {
		t.Fatalf("expected request id from header, got %q", got)
	}
	if rec.Header().Get("X-Request-ID") != "rid-header" {
		t.Fatalf("expected response header to match request id")
	}
}

func TestMiddleware_PropagatesRequestIDFromContext(t *testing.T) {
	var got string
	handler := HTTPMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := logx.RequestID(r.Context())
		if !ok {
			t.Fatalf("expected request id in context")
		}
		got = id
	}))

	req := httptest.NewRequest("GET", "/ctx", nil)
	req = req.WithContext(logx.WithRequestID(req.Context(), "rid-ctx"))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if got != "rid-ctx" {
		t.Fatalf("expected request id from context, got %q", got)
	}
	if rec.Header().Get("X-Request-ID") != "rid-ctx" {
		t.Fatalf("expected response header to match request id")
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

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rec.Code)
		}
	})

	if !strings.Contains(out, "http handler panic") {
		t.Fatalf("expected panic log, got: %s", out)
	}
	if !strings.Contains(out, "status=500") {
		t.Fatalf("expected panic completion log, got: %s", out)
	}
	if strings.Contains(out, "stack=") {
		t.Fatalf("expected middleware not to add an explicit stack field, got: %s", out)
	}
}
