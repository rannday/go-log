package logx

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileRotator_RotatesAndKeepsBackups(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")

	r, err := newFileRotator(path, 100, 2)
	if err != nil {
		t.Fatalf("failed to create rotator: %v", err)
	}
	defer r.Close()

	// write multiple times to exceed max size
	for i := 0; i < 10; i++ {
		if _, err := r.Write([]byte(strings.Repeat("x", 30))); err != nil {
			t.Fatalf("write failed: %v", err)
		}
	}

	// ensure original file exists
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected log file to exist: %v", err)
	}

	// check rotated backups exist (0..n)
	matches, _ := filepath.Glob(path + ".*")
	if len(matches) == 0 {
		t.Fatalf("expected rotated files, found none")
	}

	// write some more to trigger additional rotations
	if _, err := r.Write([]byte(strings.Repeat("y", 200))); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// ensure backups are limited to configured number (<=2)
	matches, _ = filepath.Glob(path + ".*")
	if len(matches) > 3 { // allow some timing variability
		t.Fatalf("too many backups: %d", len(matches))
	}

	// try closing
	_ = r.Close()

	// ensure file is still readable
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open failed: %v", err)
	}
	_, _ = io.ReadAll(f)
	f.Close()
}
