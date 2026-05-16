package logx

import (
	"log/slog"
	"testing"
)

func TestWithLogger_And_LoggerFromContext(t *testing.T) {
	l := slog.New(slog.NewTextHandler(&nopWriter{}, nil))
	ctx := WithLogger(nil, l)
	got := LoggerFromContext(ctx)
	if got != l {
		t.Fatalf("expected logger from context to match")
	}
}

// nopWriter implements io.Writer but does nothing.
type nopWriter struct{}

func (n *nopWriter) Write(p []byte) (int, error) { return len(p), nil }
