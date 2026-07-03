package httpx

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rannday/go-log"
)

type benchRoundTripper func(*http.Request) (*http.Response, error)

func (f benchRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func BenchmarkHTTPMiddlewareBasicRequest(b *testing.B) {
	logx.Reset()
	defer logx.Reset()
	logx.SetLogger(slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{AddSource: false})))

	handler := HTTPMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/bench", nil)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}
}

func BenchmarkHTTPTransportBasicRequest(b *testing.B) {
	logx.Reset()
	defer logx.Reset()
	logx.SetLogger(slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{AddSource: false})))

	rt := benchRoundTripper(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("ok")),
		}, nil
	})

	client := &http.Client{Transport: Transport(rt)}
	req := httptest.NewRequest(http.MethodGet, "https://example.com", nil)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = client.Do(req)
	}
}

func BenchmarkNewTransportLoggerBasicRequest(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{AddSource: false}))
	rt := benchRoundTripper(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("ok")),
		}, nil
	})

	client := &http.Client{Transport: NewTransportLogger(rt, logger)}
	req := httptest.NewRequest(http.MethodGet, "https://example.com", nil)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = client.Do(req)
	}
}
