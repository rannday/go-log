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
	defer ClearRedactedKeys()
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
