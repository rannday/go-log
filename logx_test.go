package logx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func capture(t *testing.T, level slog.Level, fn func()) string {
	t.Helper()

	Reset()

	var buf bytes.Buffer

	levelVar.Set(level)
	useColor = false

	base := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level:     levelVar,
		AddSource: false,
	})

	// Match Configure() decorator chain
	handler := newStackHandler(base, 0, false)
	handler = newRedactionHandler(handler)

	logger = slog.New(handler)

	fn()
	return buf.String()
}

func assertContains(t *testing.T, out, want string) {
	t.Helper()
	if !strings.Contains(out, want) {
		t.Fatalf("expected %q in output, got: %q", want, out)
	}
}

//
// ---- Basic Level Tests ----
//

func TestDebug_SuppressedWhenDisabled(t *testing.T) {
	out := capture(t, slog.LevelInfo, func() {
		Debug("hello", "n", 1)
	})

	if out != "" {
		t.Fatalf("expected no output, got: %q", out)
	}
}

func TestDebug_EmitsWhenEnabled(t *testing.T) {
	out := capture(t, slog.LevelDebug, func() {
		Debug("hello", "n", 1)
	})

	assertContains(t, out, "level=DEBUG")
	assertContains(t, out, "hello")
	assertContains(t, out, "n=1")
}

func TestInfo_Emits(t *testing.T) {
	out := capture(t, slog.LevelInfo, func() {
		Info("hi", "who", "there")
	})

	assertContains(t, out, "level=INFO")
	assertContains(t, out, "who=there")
}

func TestWarn_Emits(t *testing.T) {
	out := capture(t, slog.LevelInfo, func() {
		Warn("careful", "n", 2)
	})

	assertContains(t, out, "level=WARN")
}

func TestError_Emits(t *testing.T) {
	out := capture(t, slog.LevelInfo, func() {
		Error("bad", "thing", true)
	})

	assertContains(t, out, "level=ERROR")
	assertContains(t, out, "thing=true")
}

//
// ---- Structured Tests ----
//

func TestWith_AddsFields(t *testing.T) {
	out := capture(t, slog.LevelInfo, func() {
		log := With("component", "api")
		log.Info("request", "method", "GET")
	})

	assertContains(t, out, "component=api")
	assertContains(t, out, "method=GET")
}

func TestInfoContext_Emits(t *testing.T) {
	out := capture(t, slog.LevelInfo, func() {
		ctx := context.Background()
		InfoContext(ctx, "ctx message", "k", "v")
	})

	assertContains(t, out, "ctx message")
	assertContains(t, out, "k=v")
}

//
// ---- Runtime Level Test ----
//

func TestSetLevel_RuntimeChange(t *testing.T) {
	Reset()

	var buf bytes.Buffer
	useColor = false

	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level:     levelVar,
		AddSource: false,
	})

	logger = slog.New(handler)

	SetLevel(slog.LevelError)

	Info("should not print")
	if buf.String() != "" {
		t.Fatalf("expected no output, got: %q", buf.String())
	}

	SetLevel(slog.LevelInfo)

	Info("now prints")
	assertContains(t, buf.String(), "now prints")
}

//
// ---- Hardened Init Test ----
//

func TestConfigure_FileFailureFallback(t *testing.T) {
	Reset()

	// Force console off and invalid file path
	err := Configure(Config{
		Level:    slog.LevelInfo,
		Console:  false,
		FilePath: "/invalid/path/should/fail.log",
	})

	if err == nil {
		t.Fatalf("expected error for invalid file path")
	}

	// Replace logger with buffer to avoid noisy stderr
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level:     levelVar,
		AddSource: false,
	})
	logger = slog.New(handler)

	// Should not panic
	Info("still works")

	if !strings.Contains(buf.String(), "still works") {
		t.Fatalf("expected fallback logger to work")
	}
}

//
// ---- Fatal Test ----
//

func TestFatal_ExitsWithCode1AndLogs(t *testing.T) {
	if os.Getenv("LOGX_FATAL_CHILD") == "1" {
		Reset()
		useColor = false

		handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level:     levelVar,
			AddSource: false,
		})
		logger = slog.New(handler)

		Fatal("boom", "n", 2)
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestFatal_ExitsWithCode1AndLogs")
	cmd.Env = append(os.Environ(), "LOGX_FATAL_CHILD=1")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	out := stderr.String()

	if err == nil {
		t.Fatalf("expected exit code 1")
	}

	assertContains(t, out, "level=ERROR")
	assertContains(t, out, "boom")
	assertContains(t, out, "n=2")
}

func TestErrorErr_AddsErrorFields(t *testing.T) {
	out := capture(t, slog.LevelInfo, func() {
		err := fmt.Errorf("boom")
		ErrorErr("failed", err, "device", "fw1")
	})

	assertContains(t, out, "level=ERROR")
	assertContains(t, out, "failed")
	assertContains(t, out, "error=boom")
	assertContains(t, out, "error_type=*errors.errorString")
	assertContains(t, out, "device=fw1")
}

func TestErrorErrContext_AddsErrorFields(t *testing.T) {
	out := capture(t, slog.LevelInfo, func() {
		ctx := context.Background()
		err := fmt.Errorf("ctxboom")
		ErrorErrContext(ctx, "ctx failed", err)
	})

	assertContains(t, out, "ctx failed")
	assertContains(t, out, "error=ctxboom")
}

func TestTimedLevel_EmitsStartAndComplete(t *testing.T) {
	out := capture(t, slog.LevelInfo, func() {
		ctx := context.Background()

		done := TimedLevel(Logger(), slog.LevelInfo, ctx,
			"operation",
			"id", 1,
		)

		done()
	})

	assertContains(t, out, "operation started")
	assertContains(t, out, "operation completed")
	assertContains(t, out, "duration=")
}

func TestDetectColor_NoColorEnv(t *testing.T) {
	os.Setenv("NO_COLOR", "1")
	defer os.Unsetenv("NO_COLOR")
	if detectColor() {
		t.Fatalf("expected detectColor to be false when NO_COLOR is set")
	}
}

func TestMultiHandler_EnabledAndWithAttrs(t *testing.T) {
	hFalse := &testHandler{enabled: false}
	hTrue := &testHandler{enabled: true}
	m := newMultiHandler(hFalse, hTrue)
	if !m.Enabled(context.Background(), slog.LevelInfo) {
		t.Fatalf("expected multiHandler to be enabled when one handler is enabled")
	}

	h2 := m.WithAttrs([]slog.Attr{{Key: "k", Value: slog.StringValue("v")}})
	if h2 == nil {
		t.Fatalf("expected WithAttrs to return a handler")
	}
}

func TestTimedWith_LogsStartAndComplete(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{AddSource: false})
	l := slog.New(handler)

	ctx := context.Background()
	done := TimedWith(l, ctx, "op", "id", 1)
	time.Sleep(1 * time.Millisecond)
	done()

	out := buf.String()
	if out == "" {
		t.Fatalf("expected logs from TimedWith, got empty")
	}
}

func TestNew_DoesNotChangeGlobalLogger(t *testing.T) {
	Reset()
	defer Reset()

	var globalBuf bytes.Buffer
	SetLogger(slog.New(slog.NewTextHandler(&globalBuf, &slog.HandlerOptions{AddSource: false})))
	before := Logger()

	writer := &trackingWriteCloser{}
	l, closer, err := New(Config{
		Level:      slog.LevelInfo,
		Console:    false,
		FileWriter: writer,
	})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	if l == nil {
		t.Fatalf("expected logger")
	}
	if closer == nil {
		t.Fatalf("expected closer")
	}
	if got := Logger(); got != before {
		t.Fatalf("expected global logger to stay unchanged")
	}

	Info("global message")
	if !strings.Contains(globalBuf.String(), "global message") {
		t.Fatalf("expected global logger to keep working")
	}
	if writer.String() != "" {
		t.Fatalf("expected New to keep writer detached from globals")
	}

	if err := closer.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}
}

func TestNewCloser_IsIdempotent(t *testing.T) {
	Reset()
	defer Reset()

	_, closer, err := New(Config{
		Level:      slog.LevelInfo,
		Console:    false,
		FileWriter: &trackingWriteCloser{},
	})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	if err := closer.Close(); err != nil {
		t.Fatalf("first close failed: %v", err)
	}
	if err := closer.Close(); err != nil {
		t.Fatalf("second close failed: %v", err)
	}
}

func TestNew_FailedDoesNotAffectGlobalState(t *testing.T) {
	Reset()
	defer Reset()

	var globalBuf bytes.Buffer
	SetLogger(slog.New(slog.NewTextHandler(&globalBuf, &slog.HandlerOptions{AddSource: false})))
	before := Logger()

	_, closer, err := New(Config{
		Level:    slog.LevelInfo,
		Console:  false,
		FilePath: "/invalid/path/should/fail.log",
	})
	if err == nil {
		t.Fatalf("expected New to fail")
	}
	if closer != nil {
		t.Fatalf("expected nil closer on failure")
	}
	if got := Logger(); got != before {
		t.Fatalf("expected global logger to stay unchanged")
	}
	Info("still works")
	if !strings.Contains(globalBuf.String(), "still works") {
		t.Fatalf("expected global logger to keep working")
	}
}

func TestConfigure_FileFailureWithConsoleEnabled(t *testing.T) {
	Reset()
	defer Reset()

	var buf bytes.Buffer
	SetLogger(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{AddSource: false})))
	before := Logger()

	err := Configure(Config{
		Level:    slog.LevelInfo,
		Console:  true,
		FilePath: "/invalid/path/should/fail.log",
	})
	if err == nil {
		t.Fatalf("expected Configure to fail")
	}
	if got := Logger(); got != before {
		t.Fatalf("expected logger to stay unchanged")
	}
	Info("still routes to old logger")
	if !strings.Contains(buf.String(), "still routes to old logger") {
		t.Fatalf("expected old logger to keep working")
	}
}

func TestConfigure_FileFailureWithConsoleDisabled(t *testing.T) {
	Reset()
	defer Reset()

	var buf bytes.Buffer
	SetLogger(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{AddSource: false})))
	before := Logger()

	err := Configure(Config{
		Level:    slog.LevelInfo,
		Console:  false,
		FilePath: "/invalid/path/should/fail.log",
	})
	if err == nil {
		t.Fatalf("expected Configure to fail")
	}
	if got := Logger(); got != before {
		t.Fatalf("expected logger to stay unchanged")
	}
	Info("still routes to old logger")
	if !strings.Contains(buf.String(), "still routes to old logger") {
		t.Fatalf("expected old logger to keep working")
	}
}

func TestConfigure_FailedReconfigureKeepsExistingLoggerAndWriterUsable(t *testing.T) {
	Reset()
	defer Reset()

	w1 := &trackingWriteCloser{}
	err := Configure(Config{
		Level:      slog.LevelInfo,
		Console:    false,
		FileWriter: w1,
	})
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	Info("first")

	err = Configure(Config{
		Level:    slog.LevelInfo,
		Console:  false,
		FilePath: "/invalid/path/should/fail.log",
	})
	if err == nil {
		t.Fatalf("expected failed reconfigure to return error")
	}
	if got := w1.CloseCount(); got != 0 {
		t.Fatalf("expected previous writer to stay open, got %d closes", got)
	}

	if _, err := w1.Write([]byte("manual")); err != nil {
		t.Fatalf("expected previous writer to remain usable: %v", err)
	}

	Info("second")
	assertContains(t, w1.String(), "second")
}

func TestConfigure_SuccessfulReconfigureClosesPreviousWriter(t *testing.T) {
	Reset()
	defer Reset()

	w1 := &trackingWriteCloser{}
	if err := Configure(Config{
		Level:      slog.LevelInfo,
		Console:    false,
		FileWriter: w1,
	}); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	w2 := &trackingWriteCloser{}
	if err := Configure(Config{
		Level:      slog.LevelInfo,
		Console:    false,
		FileWriter: w2,
	}); err != nil {
		t.Fatalf("reconfigure failed: %v", err)
	}

	if got := w1.CloseCount(); got != 1 {
		t.Fatalf("expected previous writer to be closed once, got %d", got)
	}
	if _, err := w1.Write([]byte("after-close")); err == nil {
		t.Fatalf("expected closed writer to reject writes")
	}
	Info("after")
	assertContains(t, w2.String(), "after")
}

func TestConfigure_UsesFileWriter(t *testing.T) {
	Reset()
	defer Reset()
	var buf nopWriteCloser
	buf.Buffer = &bytes.Buffer{}

	err := Configure(Config{
		Level:      slog.LevelInfo,
		Console:    false,
		FileWriter: buf,
	})
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	Logger().Info("hello", "k", "v")
	if buf.Len() == 0 {
		t.Fatalf("expected logs written to FileWriter")
	}
}

func TestConfigureTwice_NoPanic(t *testing.T) {
	Reset()
	defer Reset()

	if err := Configure(Config{
		Level:   slog.LevelInfo,
		Console: true,
	}); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	file := filepath.Join(t.TempDir(), "app.log")
	if err := Configure(Config{
		Level:    slog.LevelInfo,
		Console:  false,
		FilePath: file,
	}); err != nil {
		t.Fatalf("Reconfigure failed: %v", err)
	}

	Info("still-running")
}

func TestReset_ClearsStateAndClosesWriter(t *testing.T) {
	Reset()
	defer Reset()

	w := &trackingWriteCloser{}
	if err := Configure(Config{
		Level:      slog.LevelInfo,
		Console:    false,
		FileWriter: w,
	}); err != nil {
		t.Fatalf("Reconfigure failed: %v", err)
	}

	Info("before-reset")
	Reset()

	if got := w.CloseCount(); got != 1 {
		t.Fatalf("expected writer close once on reset, got %d", got)
	}
	if logger != nil {
		t.Fatalf("expected logger to be nil after reset")
	}
	if currentCloser != nil {
		t.Fatalf("expected currentCloser to be nil after reset")
	}
	SetLogger(slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{AddSource: false})))
	Info("after-reset")
	if strings.Contains(w.String(), "after-reset") {
		t.Fatalf("expected old writer to be detached after reset")
	}
}

func TestMultiHandler_ReturnsFirstError(t *testing.T) {
	e1 := errors.New("first")
	h := newMultiHandler(&errHandler{err: e1}, &errHandler{err: nil})

	rec := slog.NewRecord(time.Now(), slog.LevelInfo, "m", 0)
	if err := h.Handle(context.Background(), rec); err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestMultiHandler_WithGroupAndWithAttrs(t *testing.T) {
	s1 := &simpleHandler{}
	s2 := &simpleHandler{}
	m := newMultiHandler(s1, s2)

	wg := m.WithGroup("g")
	if wg == nil {
		t.Fatalf("expected WithGroup to return a handler")
	}

	wa := m.WithAttrs([]slog.Attr{{Key: "k", Value: slog.StringValue("v")}})
	if wa == nil {
		t.Fatalf("expected WithAttrs to return a handler")
	}
}

func TestColorWriter_ReplacesLevelColor(t *testing.T) {
	var buf bytes.Buffer
	cw := &colorWriter{w: &buf}

	line := "time=now level=ERROR msg=oops\n"
	_, err := cw.Write([]byte(line))
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "level=ERROR") {
		t.Fatalf("expected level text present")
	}
	if !strings.Contains(out, "\033[31m") {
		t.Fatalf("expected red color escape in output")
	}
}

type nopWriteCloser struct{ *bytes.Buffer }

func (n nopWriteCloser) Close() error { return nil }

type trackingWriteCloser struct {
	mu         sync.Mutex
	buf        bytes.Buffer
	closed     bool
	closeCount int
}

func (t *trackingWriteCloser) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return 0, errors.New("writer is closed")
	}
	return t.buf.Write(p)
}

func (t *trackingWriteCloser) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.closed = true
	t.closeCount++
	return nil
}

func (t *trackingWriteCloser) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.buf.String()
}

func (t *trackingWriteCloser) CloseCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.closeCount
}

type testHandler struct{ enabled bool }

func (t *testHandler) Enabled(ctx context.Context, level slog.Level) bool { return t.enabled }
func (t *testHandler) Handle(ctx context.Context, r slog.Record) error    { return nil }
func (t *testHandler) WithAttrs(attrs []slog.Attr) slog.Handler           { return t }
func (t *testHandler) WithGroup(name string) slog.Handler                 { return t }

type errHandler struct {
	err error
}

func (e *errHandler) Enabled(ctx context.Context, level slog.Level) bool { return true }
func (e *errHandler) Handle(ctx context.Context, r slog.Record) error    { return e.err }
func (e *errHandler) WithAttrs(attrs []slog.Attr) slog.Handler           { return e }
func (e *errHandler) WithGroup(name string) slog.Handler                 { return e }

type simpleHandler struct{}

func (s *simpleHandler) Enabled(ctx context.Context, level slog.Level) bool { return true }
func (s *simpleHandler) Handle(ctx context.Context, r slog.Record) error    { return nil }
func (s *simpleHandler) WithAttrs(attrs []slog.Attr) slog.Handler           { return s }
func (s *simpleHandler) WithGroup(name string) slog.Handler                 { return s }
