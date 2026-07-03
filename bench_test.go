package logx

import (
	"context"
	"io"
	"log/slog"
	"net/url"
	"testing"
)

func BenchmarkDebugDisabled(b *testing.B) {
	b.ReportAllocs()
	Reset()
	defer Reset()

	SetLogger(slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: false,
	})))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Debug("debug", "n", 1)
	}
}

func BenchmarkInfoLog(b *testing.B) {
	b.ReportAllocs()
	Reset()
	defer Reset()

	SetLogger(slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: false,
	})))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Info("info", "n", 1)
	}
}

func BenchmarkTimedHelper(b *testing.B) {
	b.ReportAllocs()
	Reset()
	defer Reset()

	SetLogger(slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: false,
	})))

	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		done := Timed(ctx, "operation", "id", 1)
		done()
	}
}

func BenchmarkErrorStackDisabled(b *testing.B) {
	b.ReportAllocs()
	Reset()
	defer Reset()

	base := slog.NewTextHandler(io.Discard, &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: false,
	})
	SetLogger(slog.New(newStackHandler(base, slog.LevelError, false)))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Error("boom", "n", 1)
	}
}

func BenchmarkErrorStackEnabled(b *testing.B) {
	b.ReportAllocs()
	Reset()
	defer Reset()
	defer SetStackMaxBytes(defaultStackMaxBytes)

	base := slog.NewTextHandler(io.Discard, &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: false,
	})
	SetLogger(slog.New(newStackHandler(base, slog.LevelError, true)))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Error("boom", "n", 1)
	}
}

func BenchmarkRedaction(b *testing.B) {
	b.ReportAllocs()
	Reset()
	defer Reset()
	defer ClearRedactedKeys()

	handler := slog.NewTextHandler(io.Discard, &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: false,
	})

	b.Run("no_keys", func(b *testing.B) {
		ClearRedactedKeys()
		SetLogger(slog.New(newRedactionHandler(handler)))
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			Info("login", "user", "alice", "role", "admin")
		}
	})

	b.Run("keys_no_match", func(b *testing.B) {
		SetRedactedKeys("password", "token")
		SetLogger(slog.New(newRedactionHandler(handler)))
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			Info("login", "user", "alice", "role", "admin")
		}
	})

	b.Run("top_level_match", func(b *testing.B) {
		SetRedactedKeys("password", "token")
		SetLogger(slog.New(newRedactionHandler(handler)))
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			Info("login", "password", "secret", "user", "alice")
		}
	})

	b.Run("nested_group_match", func(b *testing.B) {
		SetRedactedKeys("token")
		SetLogger(slog.New(newRedactionHandler(handler)))
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			Info("login", "auth", slog.Group("nested",
				slog.String("token", "secret"),
				slog.String("state", "ok"),
			))
		}
	})

	b.Run("url_sanitize", func(b *testing.B) {
		u, _ := url.Parse("https://example.com/api?apikey=secret&name=test")
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = SanitizeURL(u)
		}
	})
}
