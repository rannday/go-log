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

const redactedValue = "REDACTED"

var defaultURLRedactKeys = []string{"apikey", "password", "token", "key"}

var (
	redactedKeys         = defaultRedactedKeySet()
	redactedKeysMu       sync.RWMutex
	redactedKeysSnapshot atomic.Value // map[string]struct{}
)

func init() {
	redactedKeysSnapshot.Store(cloneKeySet(redactedKeys))
}

// SetRedactedKeys replaces the global structured redaction set exactly.
// Keys are normalized to lowercase. Built-in defaults are not retained unless
// they are passed explicitly.
func SetRedactedKeys(keys ...string) {
	redactedKeysMu.Lock()
	defer redactedKeysMu.Unlock()
	redactedKeys = make(map[string]struct{}, len(keys))
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

// ClearRedactedKeys removes all structured redaction keys, including defaults.
func ClearRedactedKeys() {
	redactedKeysMu.Lock()
	defer redactedKeysMu.Unlock()
	redactedKeys = map[string]struct{}{}
	redactedKeysSnapshot.Store(map[string]struct{}{})
}

func resetRedactedKeysToDefault() {
	redactedKeysMu.Lock()
	defer redactedKeysMu.Unlock()
	redactedKeys = defaultRedactedKeySet()
	redactedKeysSnapshot.Store(cloneKeySet(redactedKeys))
}

// RedactedKeySet returns a snapshot of configured redacted keys as a set.
// The returned map is a clone and can be safely mutated by callers.
func RedactedKeySet() map[string]struct{} {
	return cloneKeySet(redactedKeySetSnapshot())
}

func redactedKeySetSnapshot() map[string]struct{} {
	keys, _ := redactedKeysSnapshot.Load().(map[string]struct{})
	return keys
}

// ListRedactedKeys returns a sorted snapshot of configured redacted keys.
func ListRedactedKeys() []string {
	keys := RedactedKeySet()
	out := make([]string, 0, len(keys))
	for k := range keys {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// RedactQueryValues masks values for keys present in keys (case-insensitive).
func RedactQueryValues(q url.Values, keys map[string]struct{}) {
	if len(keys) == 0 || len(q) == 0 {
		return
	}
	for k := range q {
		if _, ok := keys[strings.ToLower(k)]; ok {
			q.Set(k, redactedValue)
		}
	}
}

// SanitizeURL returns a redacted URL string with sensitive query parameters
// and user credentials masked. Query redaction uses SetRedactedKeys plus a
// built-in default set (apikey, password, token, key).
func SanitizeURL(u *url.URL) string {
	if u == nil {
		return ""
	}

	clone := *u
	if clone.User != nil {
		clone.User = url.UserPassword(redactedValue, redactedValue)
	}

	q := clone.Query()
	RedactQueryValues(q, urlRedactionKeySet())
	clone.RawQuery = q.Encode()
	return clone.String()
}

func urlRedactionKeySet() map[string]struct{} {
	configured := redactedKeySetSnapshot()
	merged := make(map[string]struct{}, len(configured)+len(defaultURLRedactKeys))
	for k := range configured {
		merged[k] = struct{}{}
	}
	for _, k := range defaultURLRedactKeys {
		merged[k] = struct{}{}
	}
	return merged
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
	keys := redactedKeySetSnapshot()
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

func defaultRedactedKeySet() map[string]struct{} {
	keys := make(map[string]struct{}, len(defaultURLRedactKeys))
	for _, k := range defaultURLRedactKeys {
		keys[k] = struct{}{}
	}
	return keys
}

func redactAttr(a slog.Attr, keys map[string]struct{}) (slog.Attr, bool) {
	if _, ok := keys[strings.ToLower(a.Key)]; ok {
		a.Value = slog.StringValue(redactedValue)
		return a, true
	}

	switch a.Value.Kind() {
	case slog.KindGroup:
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
	case slog.KindAny:
		if changed, v := redactAnyValue(a.Value.Any(), keys); changed {
			a.Value = slog.AnyValue(v)
			return a, true
		}
	}

	return a, false
}

func redactAnyValue(v any, keys map[string]struct{}) (bool, any) {
	switch x := v.(type) {
	case map[string]any:
		changed := false
		var redacted map[string]any
		for k, child := range x {
			if _, ok := keys[strings.ToLower(k)]; ok {
				if redacted == nil {
					redacted = cloneAnyMap(x)
				}
				redacted[k] = redactedValue
				changed = true
				continue
			}
			if c, next := redactAnyValue(child, keys); c {
				if redacted == nil {
					redacted = cloneAnyMap(x)
				}
				redacted[k] = next
				changed = true
			}
		}
		if !changed {
			return false, v
		}
		return true, redacted
	case []any:
		changed := false
		var redacted []any
		for i, child := range x {
			if c, next := redactAnyValue(child, keys); c {
				if redacted == nil {
					redacted = cloneAnySlice(x)
				}
				redacted[i] = next
				changed = true
			}
		}
		if !changed {
			return false, v
		}
		return true, redacted
	default:
		return false, v
	}
}

func cloneAnyMap(src map[string]any) map[string]any {
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func cloneAnySlice(src []any) []any {
	dst := make([]any, len(src))
	copy(dst, src)
	return dst
}
