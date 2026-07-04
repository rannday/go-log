# Changelog

## Unreleased

### Changed

- Context helpers (`InfoContext`, `ErrorErrContext`, `Timed`, and friends) use `LoggerFromContext`
- `SanitizeURL` honors `SetRedactedKeys` plus built-in defaults and redacts URL userinfo
- `Transport` is now a thin wrapper around `NewTransportLogger`
- Rotated log files use the `.logx-rotated-` suffix; oversized writes rotate after landing
- `multiHandler` skips handlers that are disabled for the record level

### Added

- `WrapHandler` for composing stack-trace and redaction decorators without globals
- `RedactedKeySet` and `RedactQueryValues` for shared redaction lookups
- `httpx.HTTPStatusLevel` for consistent HTTP status-to-level mapping

## v0.1.0

Initial release of `github.com/rannday/go-log`.

### Added

- `logx.Configure` for global logger setup
- `logx.New` for standalone logger construction
- Stack traces by level
- Runtime level changes
- Key-based redaction and URL sanitizing
- Request ID helpers
- Timing helpers
- Size-based file rotation with backup pruning
- Text-console color output only
- `httpx` server middleware
- `httpx` client transport logging
- Examples and benchmarks

### Known limits

- File rotation is size-based only
- Rotation is intentionally simple, not full production log-rotation system
- Color output is text-console only
- HTTP body logging skips large or unsupported bodies by design
