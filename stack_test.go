package logx

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
)

func TestStacktraceLevel_AddsStack(t *testing.T) {
	Reset()

	var buf bytes.Buffer
	useColor = false

	err := Configure(Config{
		Level:             slog.LevelInfo,
		Console:           false,
		StacktraceLevel:   slog.LevelError,
		StacktraceEnabled: true,
	})

	if err != nil {
		t.Fatalf("unexpected configure error: %v", err)
	}

	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level:     levelVar,
		AddSource: false,
	})
	logger = slog.New(newStackHandler(handler, slog.LevelError, true))

	Error("boom")

	out := buf.String()

	if out == "" {
		t.Fatalf("expected output, got empty")
	}
}

type passthroughHandler struct{}

func (p *passthroughHandler) Enabled(ctx context.Context, level slog.Level) bool { return true }
func (p *passthroughHandler) Handle(ctx context.Context, r slog.Record) error    { return nil }
func (p *passthroughHandler) WithAttrs(attrs []slog.Attr) slog.Handler           { return p }
func (p *passthroughHandler) WithGroup(name string) slog.Handler                 { return p }

func TestNewStackHandler_WithDelegation(t *testing.T) {
	p := &passthroughHandler{}
	h := newStackHandler(p, slog.LevelError, true)
	if h == nil {
		t.Fatalf("expected stack handler")
	}

	if h.WithAttrs(nil) == nil {
		t.Fatalf("WithAttrs returned nil")
	}
	if h.WithGroup("g") == nil {
		t.Fatalf("WithGroup returned nil")
	}
}

func TestNewStackHandler_Disabled(t *testing.T) {
	p := &passthroughHandler{}
	h := newStackHandler(p, slog.LevelInfo, false)
	if h != p {
		t.Fatalf("expected passthrough handler when disabled")
	}
}
