// Package logx provides zero-dependency structured logging on top of slog.
//
// Configure installs package-global logging for the convenience helpers.
// New builds a standalone logger without touching global state.
// WrapHandler composes stack-trace and redaction decorators without globals.
//
// Context-aware helpers (InfoContext, Timed, and ErrorErrContext) use
// LoggerFromContext, so request-scoped loggers installed by httpx middleware
// or logx.WithLogger are picked up automatically. This is separate from slog's
// own context logger APIs in the standard library.
//
// File rotation is size-based only. Color output is limited to text console
// output and matches slog text "level=LEVEL" formatting. HTTP helpers live
// in the httpx subpackage.
package logx