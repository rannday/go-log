# go-log

Structured logging built on Go's `log/slog`.

- Text or JSON output
- Stack traces by level
- Runtime level changes
- Redaction support
- HTTP server middleware
- HTTP client transport

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

- Enabled automatically for TTY
- Disabled when piped
- Disabled if `NO_COLOR` is set

## Fatal

```go
logx.Fatal("unrecoverable error")
```

Logs at error level and exits with status code `1`.

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

### HTTP Client Transport

```go
client := &http.Client{
	Transport: httpx.Transport(nil),
}
```

## Redaction

```go
logx.SetRedactedKeys("password", "apikey", "token")
```

Example output:

```text
password=REDACTED
```

Query parameters like `apikey`, `password`, `token`, and `key` are automatically redacted in URLs.
