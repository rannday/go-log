package logx

// stack.go provides a slog.Handler that appends truncated stack traces
// to log records when the record level is at or above the configured level.

import (
	"context"
	"log/slog"
	"runtime/debug"
	"sync/atomic"
)

type stackHandler struct {
	next    slog.Handler
	level   slog.Level
	enabled bool
}

const defaultStackMaxBytes = 64 * 1024

var stackMaxBytes atomic.Int64

func init() {
	stackMaxBytes.Store(defaultStackMaxBytes)
}

// SetStackMaxBytes configures the maximum bytes of the stack trace attached to records.
// Intended for tests or advanced tuning; concurrent logging is safe.
func SetStackMaxBytes(n int) {
	if n <= 0 {
		return
	}
	stackMaxBytes.Store(int64(n))
}

func currentStackMaxBytes() int {
	n := stackMaxBytes.Load()
	if n <= 0 {
		return defaultStackMaxBytes
	}
	if n > int64(^uint(0)>>1) {
		return int(^uint(0) >> 1)
	}
	return int(n)
}

func newStackHandler(next slog.Handler, level slog.Level, enabled bool) slog.Handler {
	// If stack traces are disabled, return the original handler.
	if !enabled {
		return next
	}
	return &stackHandler{
		next:    next,
		level:   level,
		enabled: true,
	}
}

func (h *stackHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *stackHandler) Handle(ctx context.Context, r slog.Record) error {
	if r.Level >= h.level {
		stack := debug.Stack()
		limit := currentStackMaxBytes()
		if len(stack) > limit {
			stack = stack[:limit]
		}
		r.AddAttrs(slog.String("stack", string(stack)))
	}

	return h.next.Handle(ctx, r)
}

func (h *stackHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return newStackHandler(h.next.WithAttrs(attrs), h.level, h.enabled)
}

func (h *stackHandler) WithGroup(name string) slog.Handler {
	return newStackHandler(h.next.WithGroup(name), h.level, h.enabled)
}
