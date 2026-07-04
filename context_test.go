package logx

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestWithLogger_And_LoggerFromContext(t *testing.T) {
	l := slog.New(slog.NewTextHandler(&nopWriter{}, nil))
	ctx := WithLogger(context.Background(), l)
	got := LoggerFromContext(ctx)
	if got != l {
		t.Fatalf("expected logger from context to match")
	}
}

// nopWriter implements io.Writer but does nothing.
type nopWriter struct{}

func (n *nopWriter) Write(p []byte) (int, error) { return len(p), nil }

func TestInfoContext_UsesContextLogger(t *testing.T) {
	Reset()
	defer Reset()

	var globalBuf, scopedBuf bytes.Buffer
	SetLogger(slog.New(slog.NewTextHandler(&globalBuf, &slog.HandlerOptions{AddSource: false})))

	scoped := slog.New(slog.NewTextHandler(&scopedBuf, &slog.HandlerOptions{AddSource: false}))
	ctx := WithLogger(context.Background(), scoped.With("request_id", "rid-1"))

	InfoContext(ctx, "hello", "k", "v")

	if scopedBuf.Len() == 0 {
		t.Fatalf("expected scoped logger to receive log")
	}
	if !strings.Contains(scopedBuf.String(), "request_id=rid-1") {
		t.Fatalf("expected request_id from scoped logger, got: %s", scopedBuf.String())
	}
	if globalBuf.Len() != 0 {
		t.Fatalf("expected global logger to be unused, got: %s", globalBuf.String())
	}
}
