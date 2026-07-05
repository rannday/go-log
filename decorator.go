package logx

import (
	"context"
	"log/slog"
)

// baseDecorator delegates Enabled, WithAttrs, and WithGroup for slog.Handler wrappers.
// Concrete handlers embed it and implement Handle only.
// clone rebuilds the wrapper around a new inner handler after WithAttrs/WithGroup.
type baseDecorator struct {
	next  slog.Handler
	clone func(slog.Handler) slog.Handler
}

func (b *baseDecorator) init(next slog.Handler, clone func(slog.Handler) slog.Handler) {
	b.next = next
	b.clone = clone
}

func (b *baseDecorator) Enabled(ctx context.Context, level slog.Level) bool {
	return b.next.Enabled(ctx, level)
}

func (b *baseDecorator) WithAttrs(attrs []slog.Attr) slog.Handler {
	return b.clone(b.next.WithAttrs(attrs))
}

func (b *baseDecorator) WithGroup(name string) slog.Handler {
	return b.clone(b.next.WithGroup(name))
}