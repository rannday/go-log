package httpx

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/rannday/go-log"
)

func ExampleHTTPMiddleware() {
	defer logx.Reset()
	logx.SetLogger(slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{AddSource: false})))

	handler := HTTPMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
}

type exampleRoundTripper struct{}

func (exampleRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader("ok")),
	}, nil
}

func ExampleTransport() {
	defer logx.Reset()
	logx.SetLogger(slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{AddSource: false})))

	client := &http.Client{Transport: Transport(exampleRoundTripper{})}
	req := httptest.NewRequest(http.MethodGet, "https://example.com", nil)
	_, _ = client.Do(req)
}

func ExampleNewTransportLogger() {
	defer logx.Reset()
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{AddSource: false}))
	client := &http.Client{
		Transport: NewTransportLogger(exampleRoundTripper{}, logger).EnableBodyLogging(1024),
	}

	req := httptest.NewRequest(http.MethodPost, "https://example.com", strings.NewReader(`{"token":"secret"}`))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(logx.WithRequestID(req.Context(), "rid-123"))
	_, _ = client.Do(req)
}
