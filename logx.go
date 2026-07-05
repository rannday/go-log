package logx

import (
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
	// loggerMu protects logger, levelVar, and currentCloser.
	// Logger(), Configure(), SetLevel(), SetLogger(), and Reset() are safe for concurrent use.
	logger        *slog.Logger
	levelVar      = new(slog.LevelVar)
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
	FileMaxBackups   int // number of rotated files to keep (<=0 = unlimited)
	// ConsoleJSON outputs console logs as JSON when true
	ConsoleJSON bool
	// FileWriter can be provided to control file output (overrides FilePath)
	FileWriter io.WriteCloser
}

// New builds a logger from cfg without touching package global state.
// Returned closer is nil when no file-backed writer/rotator was created.
// If non-nil, closer is safe to call more than once.
func New(cfg Config) (*slog.Logger, io.Closer, error) {
	if err := cfg.validate(); err != nil {
		return nil, nil, err
	}
	lv := new(slog.LevelVar)
	lv.Set(cfg.Level)
	return buildLogger(cfg, lv)
}

// Configure rebuilds logger handlers and installs the new global logger.
// On failure, the previous global logger and writer stay active, and previous
// closer is not touched.
func Configure(cfg Config) error {
	if err := cfg.validate(); err != nil {
		return err
	}

	nextLogger, nextCloser, nextLevelVar, _, err := buildLoggerWithState(cfg)
	if err != nil {
		return err
	}

	loggerMu.Lock()
	prevCloser := currentCloser
	levelVar = nextLevelVar
	logger = nextLogger
	currentCloser = nextCloser
	slog.SetDefault(nextLogger)
	loggerMu.Unlock()

	if prevCloser != nil {
		_ = prevCloser.Close()
	}

	return nil
}

func buildLogger(cfg Config, lv *slog.LevelVar) (*slog.Logger, io.Closer, error) {
	nextLogger, nextCloser, _, _, err := buildLoggerWithState(cfg, lv)
	return nextLogger, nextCloser, err
}

func buildLoggerWithState(cfg Config, lv ...*slog.LevelVar) (*slog.Logger, io.Closer, *slog.LevelVar, bool, error) {
	var leveler *slog.LevelVar
	if len(lv) > 0 && lv[0] != nil {
		leveler = lv[0]
	} else {
		leveler = new(slog.LevelVar)
		leveler.Set(cfg.Level)
	}

	var handlers []slog.Handler
	colorEnabled := false

	if cfg.Console {
		colorEnabled = detectColor() && !cfg.ConsoleJSON
		consoleOpts := handlerOptions(leveler, cfg.AddSource, colorEnabled)

		if cfg.ConsoleJSON {
			handlers = append(handlers, slog.NewJSONHandler(os.Stderr, consoleOpts))
		} else {
			handlers = append(handlers, slog.NewTextHandler(os.Stderr, consoleOpts))
		}
	}

	fileOpts := handlerOptions(leveler, cfg.AddSource, false)

	var fileWriter io.WriteCloser
	if cfg.FileWriter != nil {
		fileWriter = cfg.FileWriter
	} else if cfg.FilePath != "" {
		if cfg.FileMaxSizeBytes > 0 {
			r, err := newFileRotator(cfg.FilePath, cfg.FileMaxSizeBytes, cfg.FileMaxBackups)
			if err != nil {
				return nil, nil, nil, false, err
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
				return nil, nil, nil, false, err
			}
			if f != nil {
				fileWriter = f
			}
		}
	}

	if fileWriter != nil {
		fileWriter = newCloseOnceCloser(fileWriter)
		if cfg.JSONFile {
			handlers = append(handlers, slog.NewJSONHandler(fileWriter, fileOpts))
		} else {
			handlers = append(handlers, slog.NewTextHandler(fileWriter, fileOpts))
		}
	}

	if len(handlers) == 0 {
		fallbackOpts := handlerOptions(leveler, cfg.AddSource, detectColor())
		handlers = append(handlers, slog.NewTextHandler(os.Stderr, fallbackOpts))
	}

	var handler slog.Handler
	if len(handlers) == 1 {
		handler = handlers[0]
	} else {
		handler = newMultiHandler(handlers...)
	}

	handler = WrapHandler(handler, cfg)

	return slog.New(handler), fileWriter, leveler, colorEnabled, nil
}

// Reset clears package logger state and closes active file writer once.
// Intended for tests or controlled setup only, not normal application flow.
func Reset() {
	loggerMu.Lock()
	prevCloser := currentCloser
	logger = nil
	currentCloser = nil
	levelVar = new(slog.LevelVar)
	loggerMu.Unlock()
	resetRedactedKeysToDefault()

	if prevCloser != nil {
		_ = prevCloser.Close()
	}
}

// SetLogger replaces the package-global logger.
// Intended for tests or controlled setup only, not normal application flow.
// Passing nil installs a safe stderr text logger.
func SetLogger(l *slog.Logger) {
	if l == nil {
		l = slog.New(slog.NewTextHandler(os.Stderr, nil))
	}

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

// SetLevel updates current global logger level.
// Safe for concurrent use, but only affects current global logger.
func SetLevel(level slog.Level) {
	loggerMu.RLock()
	lv := levelVar
	loggerMu.RUnlock()
	if lv != nil {
		lv.Set(level)
	}
}

// Logger returns the package logger.
// If no logger has been configured yet, it initializes a default
// console logger at info level.
func Logger() *slog.Logger {
	loggerMu.Lock()
	defer loggerMu.Unlock()
	if logger == nil {
		initDefaultLoggerLocked()
	}
	return logger
}

func initDefaultLoggerLocked() {
	l, closer, lv, _, err := buildLoggerWithState(Config{
		Level:   slog.LevelInfo,
		Console: true,
	})
	if err != nil {
		return
	}
	logger = l
	currentCloser = closer
	levelVar = lv
	slog.SetDefault(l)
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

// Fatal logs at error level (there is no separate fatal level) and exits with status 1.
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

func logContext(ctx context.Context, level slog.Level, msg string, args ...any) {
	LoggerFromContext(ctx).Log(ctx, level, msg, args...)
}

// DebugContext logs a debug message with context using LoggerFromContext.
func DebugContext(ctx context.Context, msg string, args ...any) {
	logContext(ctx, slog.LevelDebug, msg, args...)
}

// InfoContext logs an info message with context using LoggerFromContext.
func InfoContext(ctx context.Context, msg string, args ...any) {
	logContext(ctx, slog.LevelInfo, msg, args...)
}

// WarnContext logs a warn message with context using LoggerFromContext.
func WarnContext(ctx context.Context, msg string, args ...any) {
	logContext(ctx, slog.LevelWarn, msg, args...)
}

// ErrorContext logs an error message with context using LoggerFromContext.
func ErrorContext(ctx context.Context, msg string, args ...any) {
	logContext(ctx, slog.LevelError, msg, args...)
}

// ErrorErrContext is the context-aware variant of ErrorErr.
func ErrorErrContext(ctx context.Context, msg string, err error, args ...any) {
	if err == nil {
		LoggerFromContext(ctx).ErrorContext(ctx, msg, args...)
		return
	}

	LoggerFromContext(ctx).ErrorContext(ctx, msg, errorFields(err, args...)...)
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

// closeOnceWriteCloser wraps a WriteCloser so Close is idempotent.
// Safe for concurrent Write; Close runs at most once.
type closeOnceWriteCloser struct {
	w    io.WriteCloser
	once sync.Once
	err  error
}

func newCloseOnceCloser(w io.WriteCloser) io.WriteCloser {
	if w == nil {
		return nil
	}
	return &closeOnceWriteCloser{w: w}
}

func (c *closeOnceWriteCloser) Write(p []byte) (int, error) {
	if c == nil || c.w == nil {
		return 0, io.ErrClosedPipe
	}
	return c.w.Write(p)
}

func (c *closeOnceWriteCloser) Close() error {
	if c == nil || c.w == nil {
		return nil
	}
	c.once.Do(func() {
		c.err = c.w.Close()
	})
	return c.err
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
		if !h.Enabled(ctx, r.Level) {
			continue
		}
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

// Timed uses LoggerFromContext for the request-scoped logger when present.
func Timed(ctx context.Context, msg string, args ...any) func(extra ...any) {
	return TimedLevel(LoggerFromContext(ctx), slog.LevelInfo, ctx, msg, args...)
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

// colorLevelReplaceAttr colors the slog level attribute for text console output.
// Applied via HandlerOptions.ReplaceAttr; not used for JSON handlers.
func colorLevelReplaceAttr(groups []string, a slog.Attr) slog.Attr {
	if len(groups) > 0 || a.Key != slog.LevelKey {
		return a
	}
	level, ok := a.Value.Any().(slog.Level)
	if !ok {
		return a
	}

	var color string
	switch {
	case level >= slog.LevelError:
		color = colorRed
	case level >= slog.LevelWarn:
		color = colorYellow
	case level >= slog.LevelInfo:
		color = colorGreen
	default:
		color = colorGray
	}

	a.Value = slog.StringValue(color + level.String() + colorReset)
	return a
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
