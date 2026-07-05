package logx

// stack.go provides a slog.Handler that appends truncated stack traces
// to log records when the record level is at or above the configured level.

import (
	"context"
	"log/slog"
	"runtime/debug"
	"sync/atomic"
	"unicode/utf8"
)

type stackHandler struct {
	baseDecorator
	level slog.Level
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
	if !enabled {
		return next
	}
	h := &stackHandler{level: level}
	h.init(next, func(n slog.Handler) slog.Handler {
		return newStackHandler(n, level, true)
	})
	return h
}

func (h *stackHandler) Handle(ctx context.Context, r slog.Record) error {
	if r.Level >= h.level {
		stack := debug.Stack()
		limit := currentStackMaxBytes()
		if len(stack) > limit {
			stack = trimStackBytes(stack, limit)
		}
		r.AddAttrs(slog.String("stack", string(stack)))
	}

	return h.next.Handle(ctx, r)
}

func trimStackBytes(stack []byte, limit int) []byte {
	if len(stack) <= limit {
		return stack
	}
	stack = stack[:limit]
	for len(stack) > 0 && !utf8.Valid(stack) {
		_, size := utf8.DecodeLastRune(stack)
		stack = stack[:len(stack)-size]
	}
	return stack
}
