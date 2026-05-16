package logx

import (
	"context"
	"log/slog"
)

type ctxKey string

const (
	// requestIDKey stores the request identifier in contexts created by this package.
	requestIDKey ctxKey = "logx_request_id"
	// loggerKey stores a request-scoped *slog.Logger in the context.
	loggerKey ctxKey = "logx_logger"
)

// WithRequestID returns a new context containing a request ID.
func WithRequestID(ctx context.Context, id string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, requestIDKey, id)
}

// RequestID returns the request ID from context, if present.
func RequestID(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}

	v := ctx.Value(requestIDKey)
	if v == nil {
		return "", false
	}

	id, ok := v.(string)
	if !ok || id == "" {
		return "", false
	}

	return id, true
}

// WithLogger returns a new context containing the provided logger.
func WithLogger(ctx context.Context, l *slog.Logger) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, loggerKey, l)
}

// LoggerFromContext returns a logger stored in the context or the global
// package logger if none is present.
func LoggerFromContext(ctx context.Context) *slog.Logger {
	if ctx == nil {
		return Logger()
	}
	v := ctx.Value(loggerKey)
	if v == nil {
		return Logger()
	}
	if l, ok := v.(*slog.Logger); ok && l != nil {
		return l
	}
	return Logger()
}
