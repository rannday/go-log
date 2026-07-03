# Changelog

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
