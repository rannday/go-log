package httpx

import (
	"log/slog"
	"testing"
)

func TestHTTPStatusLevel(t *testing.T) {
	tests := []struct {
		code int
		want slog.Level
	}{
		{200, slog.LevelInfo},
		{404, slog.LevelWarn},
		{500, slog.LevelError},
	}
	for _, tc := range tests {
		if got := HTTPStatusLevel(tc.code); got != tc.want {
			t.Fatalf("HTTPStatusLevel(%d) = %v, want %v", tc.code, got, tc.want)
		}
	}
}