package logx

import "log/slog"

// WrapHandler applies stack-trace and redaction decorators to base.
// Use this to compose the same handler chain as Configure/New without globals.
func WrapHandler(base slog.Handler, cfg Config) slog.Handler {
	h := newStackHandler(base, cfg.StacktraceLevel, cfg.StacktraceEnabled || cfg.StacktraceLevel != 0)
	return newRedactionHandler(h)
}