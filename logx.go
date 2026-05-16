// Package logx provides a simple wrapper around slog with some additional features:
package logx

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime"
	"sync"
	"time"
)

var (
	logger        *slog.Logger
	levelVar      = new(slog.LevelVar)
	useColor      bool
	loggerMu      sync.RWMutex
	currentCloser io.Closer
)

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorGreen  = "\033[32m"
	colorGray   = "\033[90m"
)

// Config controls logger construction for Configure.
type Config struct {
	// Level is the minimum enabled log level.
	Level slog.Level
	// Console enables console logging to stderr.
	Console bool
	// FilePath enables file logging to this path when FileWriter is nil.
	FilePath string
	// JSONFile enables JSON output for file logs (text otherwise).
	JSONFile bool
	// AddSource enables source file/line annotation in records.
	AddSource bool
	// StacktraceLevel appends stack traces for records at/above this level.
	StacktraceLevel slog.Level
	// StacktraceEnabled explicitly enables stack traces when StacktraceLevel is 0.
	// Set this to true if you want info-level stack traces, since slog.LevelInfo is 0.
	StacktraceEnabled bool
	// File rotation settings
	FileMaxSizeBytes int // rotate when file exceeds this many bytes (0 = disabled)
	FileMaxBackups   int // number of rotated files to keep
	// ConsoleJSON outputs console logs as JSON when true
	ConsoleJSON bool
	// FileWriter can be provided to control file output (overrides FilePath)
	FileWriter io.WriteCloser
}

// Configure rebuilds logger handlers and installs the new global logger.
// Calling Configure again replaces the current handlers and closes any
// previously configured file-backed writer after the swap.
func Configure(cfg Config) error {
	nextLogger, nextCloser, err := buildLogger(cfg)

	loggerMu.Lock()
	prevCloser := currentCloser
	levelVar.Set(cfg.Level)
	logger = nextLogger
	currentCloser = nextCloser
	slog.SetDefault(nextLogger)
	loggerMu.Unlock()

	if prevCloser != nil {
		_ = prevCloser.Close()
	}

	return err
}

func buildLogger(cfg Config) (*slog.Logger, io.Closer, error) {
	opts := &slog.HandlerOptions{
		Level:     levelVar,
		AddSource: cfg.AddSource,
	}

	var handlers []slog.Handler

	if cfg.Console {
		colorEnabled := detectColor()
		useColor = colorEnabled

		var writer io.Writer = os.Stderr
		if colorEnabled {
			writer = &colorWriter{w: os.Stderr}
		}

		if cfg.ConsoleJSON {
			handlers = append(handlers, slog.NewJSONHandler(writer, opts))
		} else {
			handlers = append(handlers, slog.NewTextHandler(writer, opts))
		}
	}

	var fileWriter io.WriteCloser
	var buildErr error
	if cfg.FileWriter != nil {
		fileWriter = cfg.FileWriter
	} else if cfg.FilePath != "" {
		if cfg.FileMaxSizeBytes > 0 {
			r, err := newFileRotator(cfg.FilePath, cfg.FileMaxSizeBytes, cfg.FileMaxBackups)
			if err != nil {
				buildErr = err
			}
			if r != nil {
				fileWriter = r
			}
		} else {
			f, err := os.OpenFile(
				cfg.FilePath,
				os.O_CREATE|os.O_APPEND|os.O_WRONLY,
				0o644,
			)
			if err != nil {
				buildErr = err
			}
			if f != nil {
				fileWriter = f
			}
		}
	}

	if fileWriter != nil {
		if cfg.JSONFile {
			handlers = append(handlers, slog.NewJSONHandler(fileWriter, opts))
		} else {
			handlers = append(handlers, slog.NewTextHandler(fileWriter, opts))
		}
	}

	if len(handlers) == 0 {
		handlers = append(handlers, slog.NewTextHandler(os.Stderr, opts))
	}

	var handler slog.Handler
	if len(handlers) == 1 {
		handler = handlers[0]
	} else {
		handler = newMultiHandler(handlers...)
	}

	handler = newStackHandler(handler, cfg.StacktraceLevel, cfg.StacktraceEnabled || cfg.StacktraceLevel != 0)
	handler = newRedactionHandler(handler)

	return slog.New(handler), fileWriter, buildErr
}

// Reset clears logger state.
// Intended for testing only.
func Reset() {
	loggerMu.Lock()
	prevCloser := currentCloser
	logger = nil
	currentCloser = nil
	useColor = false
	levelVar = new(slog.LevelVar)
	loggerMu.Unlock()
	ClearRedactedKeys()

	if prevCloser != nil {
		_ = prevCloser.Close()
	}
}

// SetLogger replaces the global logger.
// Intended for testing only.
func SetLogger(l *slog.Logger) {
	loggerMu.Lock()
	prevCloser := currentCloser
	logger = l
	currentCloser = nil
	slog.SetDefault(l)
	loggerMu.Unlock()
	if prevCloser != nil {
		_ = prevCloser.Close()
	}
}

// SetLevel updates the global minimum log level at runtime.
func SetLevel(level slog.Level) {
	levelVar.Set(level)
}

// Logger returns the package logger.
// If no logger has been configured yet, it initializes a default
// console logger at info level.
func Logger() *slog.Logger {
	loggerMu.RLock()
	l := logger
	loggerMu.RUnlock()

	if l == nil {
		_ = Configure(Config{
			Level:   slog.LevelInfo,
			Console: true,
		})

		loggerMu.RLock()
		l = logger
		loggerMu.RUnlock()
	}
	return l
}

// Debug logs a message at debug level.
func Debug(msg string, args ...any) {
	Logger().Debug(msg, args...)
}

// Info logs a message at info level.
func Info(msg string, args ...any) {
	Logger().Info(msg, args...)
}

// Warn logs a message at warn level.
func Warn(msg string, args ...any) {
	Logger().Warn(msg, args...)
}

// Error logs a message at error level.
func Error(msg string, args ...any) {
	Logger().Error(msg, args...)
}

// Fatal logs a message at error level and exits the process with status 1.
func Fatal(msg string, args ...any) {
	Logger().Error(msg, args...)
	os.Exit(1)
}

// Loggable can be implemented by custom errors to emit extra structured fields.
type Loggable interface {
	LogAttrs() []slog.Attr
}

// ErrorErr logs an error with normalized fields:
// "error", "error_type", and optional Loggable attributes.
func ErrorErr(msg string, err error, args ...any) {
	if err == nil {
		Logger().Error(msg, args...)
		return
	}

	Logger().Error(msg, errorFields(err, args...)...)
}

// DebugContext logs a debug message with context.
func DebugContext(ctx context.Context, msg string, args ...any) {
	Logger().DebugContext(ctx, msg, args...)
}

// InfoContext logs an info message with context.
func InfoContext(ctx context.Context, msg string, args ...any) {
	Logger().InfoContext(ctx, msg, args...)
}

// WarnContext logs a warn message with context.
func WarnContext(ctx context.Context, msg string, args ...any) {
	Logger().WarnContext(ctx, msg, args...)
}

// ErrorContext logs an error message with context.
func ErrorContext(ctx context.Context, msg string, args ...any) {
	Logger().ErrorContext(ctx, msg, args...)
}

// ErrorErrContext is the context-aware variant of ErrorErr.
func ErrorErrContext(ctx context.Context, msg string, err error, args ...any) {
	if err == nil {
		Logger().ErrorContext(ctx, msg, args...)
		return
	}

	Logger().ErrorContext(ctx, msg, errorFields(err, args...)...)
}

// With returns a child logger with additional structured attributes.
func With(args ...any) *slog.Logger {
	return Logger().With(args...)
}

// WithGroup returns a child logger that scopes subsequent fields under name.
func WithGroup(name string) *slog.Logger {
	return Logger().WithGroup(name)
}

type multiHandler struct {
	handlers []slog.Handler
}

func newMultiHandler(h ...slog.Handler) slog.Handler {
	return &multiHandler{handlers: h}
}

func (m *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (m *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	var firstErr error
	for _, h := range m.handlers {
		if err := h.Handle(ctx, r); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (m *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := make([]slog.Handler, 0, len(m.handlers))
	for _, h := range m.handlers {
		next = append(next, h.WithAttrs(attrs))
	}
	return newMultiHandler(next...)
}

func (m *multiHandler) WithGroup(name string) slog.Handler {
	next := make([]slog.Handler, 0, len(m.handlers))
	for _, h := range m.handlers {
		next = append(next, h.WithGroup(name))
	}
	return newMultiHandler(next...)
}

// Timed uses the default logger.
func Timed(ctx context.Context, msg string, args ...any) func(extra ...any) {
	return TimedLevel(Logger(), slog.LevelInfo, ctx, msg, args...)
}

// TimedWith uses a provided logger (supports With(), WithGroup(), etc.)
func TimedWith(l *slog.Logger, ctx context.Context, msg string, args ...any) func(extra ...any) {
	return timedLogger(l, slog.LevelInfo, ctx, msg, args...)
}

// TimedLevel logs "<msg> started" and returns a closure that logs
// "<msg> completed" with elapsed duration at the provided level.
func TimedLevel(
	l *slog.Logger,
	level slog.Level,
	ctx context.Context,
	msg string,
	args ...any,
) func(extra ...any) {
	return timedLogger(l, level, ctx, msg, args...)
}

func timedLogger(
	l *slog.Logger,
	level slog.Level,
	ctx context.Context,
	msg string,
	args ...any,
) func(extra ...any) {
	start := time.Now()
	startMsg := msg + " started"
	doneMsg := msg + " completed"

	l.Log(ctx, level, startMsg, args...)

	return func(extra ...any) {
		duration := time.Since(start)

		fields := make([]any, 0, len(args)+len(extra)+2)
		fields = append(fields, args...)
		fields = append(fields, extra...)
		fields = append(fields, "duration", duration)

		l.Log(ctx, level, doneMsg, fields...)
	}
}

func errorFields(err error, args ...any) []any {
	fields := make([]any, 0, len(args)+4)
	fields = append(fields, args...)
	fields = append(fields,
		"error", err,
		"error_type", fmt.Sprintf("%T", err),
	)

	if le, ok := err.(Loggable); ok {
		for _, attr := range le.LogAttrs() {
			fields = append(fields, attr.Key, attr.Value.Any())
		}
	}

	return fields
}

type colorWriter struct {
	w io.Writer
}

func (cw *colorWriter) Write(p []byte) (int, error) {
	var (
		levelTag []byte
		colored  []byte
	)

	switch {
	case bytes.Contains(p, []byte("level=ERROR")):
		levelTag = []byte("level=ERROR")
		colored = []byte(colorRed + "level=ERROR" + colorReset)
	case bytes.Contains(p, []byte("level=WARN")):
		levelTag = []byte("level=WARN")
		colored = []byte(colorYellow + "level=WARN" + colorReset)
	case bytes.Contains(p, []byte("level=INFO")):
		levelTag = []byte("level=INFO")
		colored = []byte(colorGreen + "level=INFO" + colorReset)
	case bytes.Contains(p, []byte("level=DEBUG")):
		levelTag = []byte("level=DEBUG")
		colored = []byte(colorGray + "level=DEBUG" + colorReset)
	default:
		return cw.w.Write(p)
	}

	i := bytes.Index(p, levelTag)
	if i < 0 {
		return cw.w.Write(p)
	}

	out := make([]byte, 0, len(p)+len(colored)-len(levelTag))
	out = append(out, p[:i]...)
	out = append(out, colored...)
	out = append(out, p[i+len(levelTag):]...)
	return cw.w.Write(out)
}

func detectColor() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}

	fi, err := os.Stderr.Stat()
	if err != nil {
		return false
	}

	if (fi.Mode() & os.ModeCharDevice) == 0 {
		return false
	}

	if runtime.GOOS != "windows" {
		return true
	}

	if os.Getenv("WT_SESSION") != "" {
		return true
	}
	if os.Getenv("TERM_PROGRAM") != "" {
		return true
	}
	if os.Getenv("ANSICON") != "" {
		return true
	}

	return false
}
