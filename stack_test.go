package logx

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"
)

type stackCaptureHandler struct {
	mu    sync.Mutex
	stack string
}

func (h *stackCaptureHandler) Enabled(ctx context.Context, level slog.Level) bool { return true }

func (h *stackCaptureHandler) Handle(ctx context.Context, r slog.Record) error {
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "stack" {
			h.mu.Lock()
			h.stack = a.Value.String()
			h.mu.Unlock()
		}
		return true
	})
	return nil
}

func (h *stackCaptureHandler) WithAttrs(attrs []slog.Attr) slog.Handler { return h }
func (h *stackCaptureHandler) WithGroup(name string) slog.Handler       { return h }

func (h *stackCaptureHandler) Stack() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.stack
}

func TestStackHandler_WithAttrsPreservesEnabled(t *testing.T) {
	cap := &stackCaptureHandler{}
	h := newStackHandler(cap, slog.LevelError, true)

	child := h.WithAttrs([]slog.Attr{{Key: "component", Value: slog.StringValue("api")}})
	slog.New(child).Error("boom")

	if cap.Stack() == "" {
		t.Fatalf("expected stack on child handler")
	}
}

func TestStackHandler_WithGroupPreservesEnabled(t *testing.T) {
	cap := &stackCaptureHandler{}
	h := newStackHandler(cap, slog.LevelError, true)

	child := h.WithGroup("group")
	slog.New(child).Error("boom")

	if cap.Stack() == "" {
		t.Fatalf("expected stack on grouped child handler")
	}
}

func TestStackHandler_WithAttrsDisabledPassThrough(t *testing.T) {
	cap := &stackCaptureHandler{}
	h := newStackHandler(cap, slog.LevelError, false)

	if got := h.WithAttrs(nil); got != cap {
		t.Fatalf("expected passthrough handler when stack disabled")
	}
}

func TestStackHandler_WithGroupDisabledPassThrough(t *testing.T) {
	cap := &stackCaptureHandler{}
	h := newStackHandler(cap, slog.LevelError, false)

	if got := h.WithGroup("group"); got != cap {
		t.Fatalf("expected passthrough handler when stack disabled")
	}
}

func TestStackMaxBytes_RaceSafeUpdates(t *testing.T) {
	Reset()
	defer Reset()
	defer SetStackMaxBytes(defaultStackMaxBytes)

	cap := &stackCaptureHandler{}
	SetLogger(slog.New(newStackHandler(cap, slog.LevelError, true)))

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(offset int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				SetStackMaxBytes(128 + offset + j)
				Error("stack update")
			}
		}(i)
	}
	wg.Wait()

	if cap.Stack() == "" {
		t.Fatalf("expected stack captured during concurrent updates")
	}
}

func TestStackTruncation(t *testing.T) {
	Reset()
	defer Reset()
	defer SetStackMaxBytes(defaultStackMaxBytes)

	SetStackMaxBytes(32)

	cap := &stackCaptureHandler{}
	SetLogger(slog.New(newStackHandler(cap, slog.LevelError, true)))

	Error("truncate me")

	stack := cap.Stack()
	if stack == "" {
		t.Fatalf("expected stack to be attached")
	}
	if len(stack) > 32 {
		t.Fatalf("expected stack to be truncated to 32 bytes, got %d: %q", len(stack), stack)
	}
	if !strings.Contains(stack, "goroutine") && len(stack) == 32 {
		t.Fatalf("expected stack content, got %q", stack)
	}
}

func TestTrimStackBytes_UTF8Safe(t *testing.T) {
	got := trimStackBytes([]byte("ab☃cd"), 4)

	if !utf8.Valid(got) {
		t.Fatalf("expected valid utf-8, got %q", got)
	}
	if string(got) != "ab" {
		t.Fatalf("expected trim before partial rune, got %q", got)
	}
}
