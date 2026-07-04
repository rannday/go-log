package httpx

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/rannday/go-log"
)

// HTTPStatusLevel maps an HTTP status code to a slog level.
func HTTPStatusLevel(statusCode int) slog.Level {
	switch {
	case statusCode >= 500:
		return slog.LevelError
	case statusCode >= 400:
		return slog.LevelWarn
	default:
		return slog.LevelInfo
	}
}

func propagateRequestID(req *http.Request) {
	if req == nil {
		return
	}
	if id, ok := logx.RequestID(req.Context()); ok && req.Header.Get("X-Request-ID") == "" {
		req.Header.Set("X-Request-ID", id)
	}
}

func roundTripLogger(req *http.Request, explicit *slog.Logger) *slog.Logger {
	if explicit != nil {
		return explicit
	}
	return logx.LoggerFromContext(req.Context())
}

func appendRequestIDField(ctx context.Context, fields []any) []any {
	if id, ok := logx.RequestID(ctx); ok {
		return append(fields, "request_id", id)
	}
	return fields
}

func logHTTPFailure(l *slog.Logger, ctx context.Context, fields []any, err error, networkError bool) {
	fields = append(fields, "error", err)
	if networkError {
		fields = append(fields, "network_error", true)
	}
	l.ErrorContext(ctx, "http request failed", fields...)
}

func logHTTPCompletion(l *slog.Logger, ctx context.Context, statusCode int, fields []any) {
	fields = append(fields, "status", statusCode)
	l.Log(ctx, HTTPStatusLevel(statusCode), "http request completed", fields...)
}