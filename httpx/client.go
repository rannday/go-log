package httpx

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/rannday/go-log"
)

type loggingTransport struct {
	next http.RoundTripper
}

// Transport wraps rt with outbound request logging and request ID propagation.
// If rt is nil, http.DefaultTransport is used.
func Transport(rt http.RoundTripper) http.RoundTripper {
	if rt == nil {
		rt = http.DefaultTransport
	}
	return &loggingTransport{next: rt}
}

func (t *loggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()

	if req == nil {
		return t.next.RoundTrip(req)
	}
	l := logx.LoggerFromContext(req.Context())

	if id, ok := logx.RequestID(req.Context()); ok && req.Header.Get("X-Request-ID") == "" {
		req.Header.Set("X-Request-ID", id)
	}

	resp, err := t.next.RoundTrip(req)
	duration := time.Since(start)

	var urlStr, host string
	if req.URL != nil {
		urlStr = logx.SanitizeURL(req.URL)
		host = req.URL.Host
	}

	fields := []any{
		"method", req.Method,
		"url", urlStr,
		"host", host,
		"duration", duration,
	}

	if id, ok := logx.RequestID(req.Context()); ok {
		fields = append(fields, "request_id", id)
	}

	if err != nil {
		fields = append(fields,
			"error", err,
			"network_error", true,
		)

		l.ErrorContext(req.Context(), "http request failed", fields...)

		return resp, err
	}

	fields = append(fields, "status", resp.StatusCode)

	level := slog.LevelInfo
	switch {
	case resp.StatusCode >= 500:
		level = slog.LevelError
	case resp.StatusCode >= 400:
		level = slog.LevelWarn
	}

	l.Log(req.Context(), level, "http request completed", fields...)

	return resp, nil
}
