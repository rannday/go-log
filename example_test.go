package logx

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
)

type discardWriteCloser struct {
	io.Writer
}

func (discardWriteCloser) Close() error { return nil }

func ExampleConfigure() {
	defer Reset()

	_ = Configure(Config{
		Level:      slog.LevelInfo,
		Console:    false,
		FileWriter: discardWriteCloser{Writer: io.Discard},
	})

	Info("starting", "version", "1.0.0")
}

func ExampleNew() {
	logger, closer, _ := New(Config{
		Level:      slog.LevelInfo,
		Console:    false,
		FileWriter: discardWriteCloser{Writer: io.Discard},
	})
	defer func() {
		if closer != nil {
			_ = closer.Close()
		}
	}()

	logger.Info("starting", "version", "1.0.0")
}

func ExampleConfigure_reconfigure() {
	defer Reset()

	_ = Configure(Config{
		Level:      slog.LevelInfo,
		Console:    false,
		FileWriter: discardWriteCloser{Writer: io.Discard},
	})

	f, _ := os.CreateTemp("", "go-log-example-*.log")
	defer os.Remove(f.Name())
	defer f.Close()

	_ = Configure(Config{
		Level:      slog.LevelDebug,
		Console:    false,
		FileWriter: f,
	})

	Debug("bootstrapped")
}

func ExampleSetRedactedKeys() {
	defer Reset()
	defer ClearRedactedKeys()
	_ = Configure(Config{
		Level:      slog.LevelInfo,
		Console:    false,
		FileWriter: discardWriteCloser{Writer: io.Discard},
	})

	SetRedactedKeys("password", "token")

	Info("login", "user", "admin", "password", "secret")
}

func ExampleRequestID() {
	ctx := WithRequestID(context.Background(), "rid-123")
	_, _ = RequestID(ctx)
}

func ExampleTimed() {
	defer Reset()

	_ = Configure(Config{
		Level:      slog.LevelInfo,
		Console:    false,
		FileWriter: discardWriteCloser{Writer: io.Discard},
	})

	done := Timed(context.Background(), "operation", "id", 1)
	done()
}

type exampleLoggableError struct {
	code string
}

func (e exampleLoggableError) Error() string { return "api failure" }
func (e exampleLoggableError) LogAttrs() []slog.Attr {
	return []slog.Attr{slog.String("code", e.code)}
}

func ExampleWrapHandler() {
	var buf bytes.Buffer
	base := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	h := WrapHandler(base, Config{StacktraceLevel: slog.LevelError})
	logger := slog.New(h)

	logger.Info("ok", "user", "alice")
	logger.Error("boom", "password", "secret")

	// Output includes redacted password and stack trace fields for errors.
}

func ExampleErrorErr() {
	defer Reset()

	_ = Configure(Config{
		Level:      slog.LevelInfo,
		Console:    false,
		FileWriter: discardWriteCloser{Writer: io.Discard},
	})

	ErrorErr("request failed", exampleLoggableError{code: "E123"})
}
