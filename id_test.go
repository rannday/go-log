package logx

import (
	"strings"
	"testing"
)

func TestNewRequestID_Format(t *testing.T) {
	id := NewRequestID()
	if !strings.Contains(id, "-") {
		t.Fatalf("expected dashes in id, got: %s", id)
	}
}
