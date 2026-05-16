package logx

// stack.go provides a slog.Handler that appends truncated stack traces
// to log records when the record level is at or above the configured level.

import (
	"context"
	"log/slog"
	"runtime/debug"
)

type stackHandler struct {
	next  slog.Handler
	level slog.Level
}

// maxStackBytes limits the size of attached stack traces. Default to 64KB.
var maxStackBytes = 64 * 1024

// SetStackMaxBytes configures the maximum bytes of the stack trace attached to records.
func SetStackMaxBytes(n int) {
	if n <= 0 {
		return
	}
	maxStackBytes = n
}

func newStackHandler(next slog.Handler, level slog.Level, enabled bool) slog.Handler {
	// If stack traces are disabled, return the original handler.
	if !enabled {
		return next
	}
	return &stackHandler{
		next:  next,
		level: level,
	}
}

func (h *stackHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *stackHandler) Handle(ctx context.Context, r slog.Record) error {
	if r.Level >= h.level {
		nr := r.Clone()
		stack := debug.Stack()
		if len(stack) > maxStackBytes {
			stack = stack[:maxStackBytes]
		}
		nr.AddAttrs(
			slog.String("stack", string(stack)),
		)
		return h.next.Handle(ctx, nr)
	}

	return h.next.Handle(ctx, r)
}

func (h *stackHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return newStackHandler(h.next.WithAttrs(attrs), h.level, true)
}

func (h *stackHandler) WithGroup(name string) slog.Handler {
	return newStackHandler(h.next.WithGroup(name), h.level, true)
}
