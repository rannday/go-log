package logx

import (
	"fmt"
	"log/slog"
)

// validate checks Config fields and returns a descriptive error on invalid input.
func (c Config) validate() error {
	if c.FileMaxSizeBytes < 0 {
		return fmt.Errorf("logx: FileMaxSizeBytes must not be negative (got %d)", c.FileMaxSizeBytes)
	}
	return nil
}

// handlerOptions builds slog.HandlerOptions for a sink.
// color applies ReplaceAttr level coloring for text console output only.
func handlerOptions(lv *slog.LevelVar, addSource, color bool) *slog.HandlerOptions {
	opts := &slog.HandlerOptions{
		Level:     lv,
		AddSource: addSource,
	}
	if color {
		opts.ReplaceAttr = colorLevelReplaceAttr
	}
	return opts
}