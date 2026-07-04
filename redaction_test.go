package logx

import (
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"testing"
)

func TestConcurrentSetRedactedKeys(t *testing.T) {
	ClearRedactedKeys()

	var wg sync.WaitGroup
	add := func(prefix string) {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			SetRedactedKeys(fmt.Sprintf("%s%d", prefix, i))
		}
	}

	wg.Add(2)
	go add("a")
	go add("b")
	wg.Wait()

	// ensure there is at least one key present
	if len(ListRedactedKeys()) == 0 {
		t.Fatalf("expected redacted keys to be present")
	}
}

func TestRedactionHandler_RedactsKeys(t *testing.T) {
	out := capture(t, slog.LevelInfo, func() {
		SetRedactedKeys("password")
		Info("login", "password", "secret", "user", "admin")
	})

	if !strings.Contains(out, "password=REDACTED") {
		t.Fatalf("expected password to be redacted, got: %s", out)
	}
}

func TestRedactionHandler_DefaultRedactsPassword(t *testing.T) {
	out := capture(t, slog.LevelInfo, func() {
		Info("login", "password", "secret")
	})

	if !strings.Contains(out, "password=REDACTED") {
		t.Fatalf("expected default password redaction, got: %s", out)
	}
	if strings.Contains(out, "secret") {
		t.Fatalf("expected secret value removed, got: %s", out)
	}
}

func TestClearRedactedKeys_DisablesStructuredRedaction(t *testing.T) {
	out := capture(t, slog.LevelInfo, func() {
		ClearRedactedKeys()
		Info("login", "password", "secret")
	})

	if !strings.Contains(out, "password=secret") {
		t.Fatalf("expected password to remain unredacted, got: %s", out)
	}
	if strings.Contains(out, "password=REDACTED") {
		t.Fatalf("expected structured redaction disabled, got: %s", out)
	}
}

func TestSetRedactedKeys_ReplacesDefaults(t *testing.T) {
	out := capture(t, slog.LevelInfo, func() {
		SetRedactedKeys("secret")
		Info("fields", "password", "pw", "secret", "value")
	})

	if !strings.Contains(out, "password=pw") {
		t.Fatalf("expected password default to be replaced, got: %s", out)
	}
	if !strings.Contains(out, "secret=REDACTED") {
		t.Fatalf("expected configured key to be redacted, got: %s", out)
	}
}

func TestRedactionHandler_RedactsMixedCaseKeys(t *testing.T) {
	out := capture(t, slog.LevelInfo, func() {
		SetRedactedKeys("password")
		Info("login", "Password", "secret", "user", "admin")
	})

	if !strings.Contains(out, "Password=REDACTED") {
		t.Fatalf("expected mixed-case password to be redacted, got: %s", out)
	}
}

func TestRedactionHandler_RedactsNestedGroup(t *testing.T) {
	out := capture(t, slog.LevelInfo, func() {
		SetRedactedKeys("token")
		Info("login", "user", "admin", slog.Group("auth", slog.Group("nested",
			slog.String("token", "secret"),
			slog.String("ok", "yes"),
		)))
	})

	if !strings.Contains(out, "token=REDACTED") {
		t.Fatalf("expected nested token to be redacted, got: %s", out)
	}
	if strings.Contains(out, "secret") {
		t.Fatalf("expected nested secret to be removed, got: %s", out)
	}
}

func TestSanitizeURL_RedactsQueryParams(t *testing.T) {
	u, _ := url.Parse("https://fw/api?apikey=abc123&name=test")

	s := SanitizeURL(u)

	if strings.Contains(s, "abc123") {
		t.Fatalf("expected apikey to be redacted")
	}

	if !strings.Contains(s, "apikey=REDACTED") {
		t.Fatalf("expected apikey=REDACTED, got: %s", s)
	}
}

func TestSanitizeURL_RedactsConfiguredKeys(t *testing.T) {
	ClearRedactedKeys()
	defer Reset()
	SetRedactedKeys("secret")

	u, _ := url.Parse("https://fw/api?secret=abc&name=test")
	s := SanitizeURL(u)

	if strings.Contains(s, "abc") {
		t.Fatalf("expected configured secret param to be redacted, got: %s", s)
	}
	if !strings.Contains(s, "secret=REDACTED") {
		t.Fatalf("expected secret=REDACTED, got: %s", s)
	}
}

func TestSanitizeURL_RedactsDefaultKeysAfterClear(t *testing.T) {
	ClearRedactedKeys()
	defer Reset()

	u, _ := url.Parse("https://fw/api?password=secret&name=test")
	s := SanitizeURL(u)

	if strings.Contains(s, "secret") {
		t.Fatalf("expected default password param to be redacted, got: %s", s)
	}
	if !strings.Contains(s, "password=REDACTED") {
		t.Fatalf("expected password=REDACTED, got: %s", s)
	}
}

func TestSanitizeURL_RedactsUserCredentials(t *testing.T) {
	u, _ := url.Parse("https://admin:pass@fw/api?name=test")
	s := SanitizeURL(u)

	if strings.Contains(s, "admin") || strings.Contains(s, "pass") {
		t.Fatalf("expected user credentials to be redacted, got: %s", s)
	}
	if !strings.Contains(s, "REDACTED:REDACTED@") {
		t.Fatalf("expected redacted userinfo, got: %s", s)
	}
}

func TestRedactionHandler_RedactsAnyMap(t *testing.T) {
	out := capture(t, slog.LevelInfo, func() {
		SetRedactedKeys("token")
		Info("payload", "data", map[string]any{"token": "secret", "ok": true})
	})

	if !strings.Contains(out, "token:REDACTED") {
		t.Fatalf("expected map token to be redacted, got: %s", out)
	}
	if strings.Contains(out, "secret") {
		t.Fatalf("expected secret value removed, got: %s", out)
	}
}

func TestRedactedKeySet_ReturnsClone(t *testing.T) {
	SetRedactedKeys("password")
	defer Reset()

	keys := RedactedKeySet()
	delete(keys, "password")
	keys["token"] = struct{}{}

	fresh := RedactedKeySet()
	if _, ok := fresh["password"]; !ok {
		t.Fatalf("expected internal redaction set to keep password")
	}
	if _, ok := fresh["token"]; ok {
		t.Fatalf("expected mutation of returned map not to affect internal state")
	}
}

func TestRedactAnyValue_DoesNotMutateMap(t *testing.T) {
	SetRedactedKeys("password")
	defer Reset()

	original := map[string]any{"password": "secret"}
	changed, redacted := redactAnyValue(original, redactedKeySetSnapshot())

	if !changed {
		t.Fatalf("expected redaction to change returned value")
	}
	if original["password"] != "secret" {
		t.Fatalf("expected original map to remain unchanged, got: %#v", original)
	}
	got := redacted.(map[string]any)
	if got["password"] != redactedValue {
		t.Fatalf("expected redacted copy, got: %#v", got)
	}
}

func TestRedactAnyValue_DoesNotMutateNestedMap(t *testing.T) {
	SetRedactedKeys("token")
	defer Reset()

	nested := map[string]any{"token": "secret"}
	original := map[string]any{"outer": nested}
	changed, redacted := redactAnyValue(original, redactedKeySetSnapshot())

	if !changed {
		t.Fatalf("expected nested redaction to change returned value")
	}
	if nested["token"] != "secret" {
		t.Fatalf("expected nested map to remain unchanged, got: %#v", nested)
	}
	got := redacted.(map[string]any)
	gotNested := got["outer"].(map[string]any)
	if gotNested["token"] != redactedValue {
		t.Fatalf("expected redacted nested copy, got: %#v", gotNested)
	}
}

func TestRedactAnyValue_DoesNotMutateSlice(t *testing.T) {
	SetRedactedKeys("token")
	defer Reset()

	child := map[string]any{"token": "secret"}
	original := []any{child}
	changed, redacted := redactAnyValue(original, redactedKeySetSnapshot())

	if !changed {
		t.Fatalf("expected slice redaction to change returned value")
	}
	if child["token"] != "secret" {
		t.Fatalf("expected original map in slice to remain unchanged, got: %#v", child)
	}
	if original[0].(map[string]any)["token"] != "secret" {
		t.Fatalf("expected original slice to remain unchanged, got: %#v", original)
	}
	got := redacted.([]any)
	gotChild := got[0].(map[string]any)
	if gotChild["token"] != redactedValue {
		t.Fatalf("expected redacted slice copy, got: %#v", got)
	}
}
