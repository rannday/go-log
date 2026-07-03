package logx

import (
	"context"
	"log/slog"
	"net/url"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

// SanitizeURL returns a redacted URL string with sensitive query parameters masked.
func SanitizeURL(u *url.URL) string {
	if u == nil {
		return ""
	}

	clone := *u
	q := clone.Query()

	for k := range q {
		lk := strings.ToLower(k)
		switch lk {
		case "apikey", "password", "token", "key":
			q.Set(k, "REDACTED")
		}
	}

	clone.RawQuery = q.Encode()
	return clone.String()
}

var (
	redactedKeys         = map[string]struct{}{}
	redactedKeysMu       sync.RWMutex
	redactedKeysSnapshot atomic.Value // map[string]struct{}
)

func init() {
	redactedKeysSnapshot.Store(map[string]struct{}{})
}

// SetRedactedKeys replaces the global redaction set.
// Keys are normalized to lowercase.
func SetRedactedKeys(keys ...string) {
	redactedKeysMu.Lock()
	defer redactedKeysMu.Unlock()
	redactedKeys = make(map[string]struct{}, len(redactedKeys)+len(keys))
	for _, k := range keys {
		redactedKeys[strings.ToLower(k)] = struct{}{}
	}
	redactedKeysSnapshot.Store(cloneKeySet(redactedKeys))
}

// AddRedactedKeys appends keys to the redaction set (concurrency-safe).
func AddRedactedKeys(keys ...string) {
	redactedKeysMu.Lock()
	defer redactedKeysMu.Unlock()
	for _, k := range keys {
		redactedKeys[strings.ToLower(k)] = struct{}{}
	}
	redactedKeysSnapshot.Store(cloneKeySet(redactedKeys))
}

// ClearRedactedKeys removes all configured redacted keys.
func ClearRedactedKeys() {
	redactedKeysMu.Lock()
	defer redactedKeysMu.Unlock()
	redactedKeys = map[string]struct{}{}
	redactedKeysSnapshot.Store(map[string]struct{}{})
}

// ListRedactedKeys returns a snapshot of configured redacted keys.
func ListRedactedKeys() []string {
	keys, _ := redactedKeysSnapshot.Load().(map[string]struct{})
	out := make([]string, 0, len(keys))
	for k := range keys {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

type redactionHandler struct {
	next slog.Handler
}

func newRedactionHandler(next slog.Handler) slog.Handler {
	return &redactionHandler{next: next}
}

func (h *redactionHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *redactionHandler) Handle(ctx context.Context, r slog.Record) error {
	keys, _ := redactedKeysSnapshot.Load().(map[string]struct{})
	if len(keys) == 0 {
		return h.next.Handle(ctx, r)
	}

	attrs := make([]slog.Attr, 0, r.NumAttrs())
	changed := false
	r.Attrs(func(a slog.Attr) bool {
		redacted, ok := redactAttr(a, keys)
		if ok {
			changed = true
		}
		attrs = append(attrs, redacted)
		return true
	})

	if !changed {
		return h.next.Handle(ctx, r)
	}

	newRec := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	newRec.AddAttrs(attrs...)

	return h.next.Handle(ctx, newRec)
}

func (h *redactionHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return newRedactionHandler(h.next.WithAttrs(attrs))
}

func (h *redactionHandler) WithGroup(name string) slog.Handler {
	return newRedactionHandler(h.next.WithGroup(name))
}

func cloneKeySet(src map[string]struct{}) map[string]struct{} {
	dst := make(map[string]struct{}, len(src))
	for k := range src {
		dst[k] = struct{}{}
	}
	return dst
}

func redactAttr(a slog.Attr, keys map[string]struct{}) (slog.Attr, bool) {
	if _, ok := keys[strings.ToLower(a.Key)]; ok {
		a.Value = slog.StringValue("REDACTED")
		return a, true
	}

	if a.Value.Kind() != slog.KindGroup {
		return a, false
	}

	group := a.Value.Group()
	redacted := make([]slog.Attr, 0, len(group))
	changed := false
	for _, child := range group {
		next, ok := redactAttr(child, keys)
		if ok {
			changed = true
		}
		redacted = append(redacted, next)
	}
	if !changed {
		return a, false
	}
	a.Value = slog.GroupValue(redacted...)
	return a, true
}
