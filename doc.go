// Package logx provides zero-dependency structured logging on top of slog.
//
// Configure installs package-global logging for the convenience helpers.
// New builds a standalone logger without touching global state.
//
// File rotation is size-based only. Color output is limited to text console
// output. HTTP helpers live in the httpx subpackage.
package logx
