package logx

import "log/slog"

// WrapHandler applies stack-trace and redaction decorators to base.
// Use this to compose the same handler chain as Configure/New without globals.
//
// Decorators are applied inside-out: base → stack traces → redaction.
// Each decorator delegates Enabled/WithAttrs/WithGroup to its inner handler.
//
// Example:
//
//	buf := &bytes.Buffer{}
//	base := slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})
//	h := logx.WrapHandler(base, logx.Config{StacktraceLevel: slog.LevelError})
//	logger := slog.New(h)
//	logger.Error("boom") // stack + redaction applied; output in buf
func WrapHandler(base slog.Handler, cfg Config) slog.Handler {
	h := newStackHandler(base, cfg.StacktraceLevel, cfg.StacktraceEnabled || cfg.StacktraceLevel != 0)
	return newRedactionHandler(h)
}