# go-log

Structured logging built on Go's `log/slog`.
Zero dependency. Go 1.26.3+ is intentional.

- Text or JSON output
- Stack traces by level
- Runtime level changes
- Redaction support
- HTTP server middleware
- HTTP client transport
- `Configure` installs package-global logger
- `New` builds logger without touching globals
- `WrapHandler` composes stack-trace and redaction decorators without globals
- File rotation is size-based only
- Color output is text-console only

## Requirements

- Go 1.26.3 or newer

## Install

```bash
go get github.com/rannday/go-log
```

## Quick Start

```go
package main

import (
	"log/slog"

	logx "github.com/rannday/go-log"
)

func main() {
	logx.Configure(logx.Config{
		Level:           slog.LevelInfo,
		Console:         true,
		AddSource:       false,
		StacktraceLevel: slog.LevelError,
	})

	logx.Info("starting", "version", "1.0.0")
	logx.Warn("cache miss", "key", "user:42")
	logx.Error("operation failed", "id", 123)
}
```

## Configuration

```go
logx.Configure(logx.Config{
	Level:           slog.LevelDebug,
	Console:         true,
	FilePath:        "app.log",
	JSONFile:        false,
	AddSource:       true,
	StacktraceLevel: slog.LevelError,
})
```

If you want stack traces at info level, set `StacktraceEnabled: true` and `StacktraceLevel: slog.LevelInfo`.

## Build Without Installing

Use `New` when you want a configured logger without touching global state.

```go
logger, closer, err := logx.New(logx.Config{
	Level:      slog.LevelInfo,
	Console:    true,
	FilePath:   "app.log",
	JSONFile:   false,
	AddSource:  true,
})
if err != nil {
	panic(err)
}
defer func() {
	if closer != nil {
		_ = closer.Close()
	}
}()

logger.Info("ready")
```

## Bootstrap Then Configure

Use `Configure` for early console logging, then call `Configure` again after app config or env is loaded.

```go
// bootstrap: console only
if err := logx.Configure(logx.Config{
	Level:   slog.LevelInfo,
	Console: true,
}); err != nil {
	panic(err)
}

// ...load env/config...

// configure again: attach file output after config is known
if err := logx.Configure(logx.Config{
	Level:           slog.LevelDebug,
	Console:         true,
	FilePath:        "app.log",
	JSONFile:        false,
	AddSource:       true,
	StacktraceLevel: slog.LevelError,
}); err != nil {
	panic(err)
}
```

Calling `Configure` again is the supported way to attach file logging after startup.
If file setup fails, `Configure` returns an error and leaves the previous global logger alone.

## Runtime Level Changes

```go
logx.SetLevel(slog.LevelDebug)
```

## Structured Logging

```go
logx.Info("user login",
	"user", "admin",
	"ip", "10.0.0.5",
)
```

## Error Helpers

```go
err := doSomething()
logx.ErrorErr("operation failed", err, "device", "fw1")
```

Context version:

```go
logx.ErrorErrContext(ctx, "commit failed", err)
```

## Custom Structured Errors

```go
type APIError struct {
	Status int
	Code   string
}

func (e APIError) LogAttrs() []slog.Attr {
	return []slog.Attr{
		slog.Int("status", e.Status),
		slog.String("code", e.Code),
	}
}
```

Usage:

```go
logx.ErrorErr("api failure", apiErr)
```

## Context Helpers

```go
ctx := logx.WithRequestID(ctx, "abc123")
id, ok := logx.RequestID(ctx)
```

Context-aware helpers (`InfoContext`, `ErrorErrContext`, `Timed`, and friends) use
`LoggerFromContext`, so request-scoped loggers from `httpx` middleware or
`logx.WithLogger` are picked up automatically.

## Timing Helpers

```go
done := logx.Timed(ctx, "panos commit", "device", "fw1")
defer done()
```

Custom level:

```go
done := logx.TimedLevel(
	logx.Logger(),
	slog.LevelDebug,
	ctx,
	"panos commit",
	"device", "fw1",
)
defer done()
```

## Color Output

- Color is only applied to text console output
- Disabled for JSON console output
- Disabled for file output
- Disabled when piped
- Disabled if `NO_COLOR` is set

## Fatal

```go
logx.Fatal("unrecoverable error")
```

Logs at error level (there is no separate fatal level) and exits with status code `1`.

## Testing

```bash
go test -race ./...
```

## Middleware

HTTP utilities live in the `httpx` subpackage.

```go
import "github.com/rannday/go-log/httpx"
```

### Server Middleware

```go
handler := httpx.HTTPMiddleware(router)
```

`HTTPMiddleware` adds timing, status logging, panic recovery, and request ID propagation.
It stores the request ID in context and mirrors it to the `X-Request-ID` response header.

### HTTP Client Transport

```go
client := &http.Client{
	Transport: httpx.Transport(nil),
}
```

`Transport` adds basic outbound request logging and request ID propagation.
For more control, use `httpx.NewTransportLogger(rt, logger)` and call `EnableBodyLogging(maxBytes)` when you want small body capture.
Body logging is size-limited, skips oversized or unsupported bodies by design, and redacts JSON or form fields using the package redaction keys.

## File Rotation

Size-based rotation is available through `Config.FileMaxSizeBytes` and `Config.FileMaxBackups`.
`FileMaxBackups <= 0` means unlimited rotated backups.
It is intentionally simple: no time-based rotation, compression, or external dependency chain.
It is not a drop-in replacement for a full production log rotation system.

## Redaction

```go
logx.SetRedactedKeys("password", "apikey", "token")
```

Example output:

```text
password=REDACTED
```

Structured field redaction defaults to `apikey`, `password`, `token`, and `key`.
`SetRedactedKeys` replaces that set exactly; `ClearRedactedKeys` disables structured field redaction.
URL sanitization uses configured keys plus built-in defaults (`apikey`, `password`, `token`, `key`), even after `ClearRedactedKeys`.
User credentials in the URL userinfo are also redacted.
