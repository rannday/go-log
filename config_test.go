package logx

import (
	"strings"
	"testing"
)

func TestConfig_Validate_RejectsNegativeFileMaxSize(t *testing.T) {
	err := Config{FileMaxSizeBytes: -1}.validate()
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if !strings.Contains(err.Error(), "FileMaxSizeBytes") {
		t.Fatalf("expected descriptive error, got: %v", err)
	}
}

func TestNew_RejectsInvalidConfig(t *testing.T) {
	_, _, err := New(Config{FileMaxSizeBytes: -1})
	if err == nil {
		t.Fatalf("expected New to reject invalid config")
	}
}