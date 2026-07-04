package logx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFileRotator_OversizedSingleWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")

	r, err := newFileRotator(path, 8, 2)
	if err != nil {
		t.Fatalf("newFileRotator failed: %v", err)
	}
	defer r.Close()

	payload := strings.Repeat("x", 32)
	n, err := r.Write([]byte(payload))
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if n != len(payload) {
		t.Fatalf("expected full write, got %d", n)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if len(data) != 0 {
		t.Fatalf("expected active file to be empty after oversized write rotation, got %d bytes", len(data))
	}

	matches, err := filepath.Glob(rotatedLogGlob(path))
	if err != nil {
		t.Fatalf("glob failed: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected one rotated backup, got %d: %v", len(matches), matches)
	}
	backup, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("read backup failed: %v", err)
	}
	if len(backup) != len(payload) {
		t.Fatalf("expected backup to contain oversized payload, got %d bytes", len(backup))
	}
}

func TestFileRotator_WriteAfterClose(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")

	r, err := newFileRotator(path, 8, 2)
	if err != nil {
		t.Fatalf("newFileRotator failed: %v", err)
	}

	if err := r.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}

	if _, err := r.Write([]byte("boom")); err == nil {
		t.Fatalf("expected write after close to fail")
	}
}

func TestFileRotator_BackupPruning(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")

	r, err := newFileRotator(path, 4, 2)
	if err != nil {
		t.Fatalf("newFileRotator failed: %v", err)
	}
	defer r.Close()

	for i := 0; i < 5; i++ {
		if _, err := r.Write([]byte("abcd")); err != nil {
			t.Fatalf("write %d failed: %v", i, err)
		}
		time.Sleep(time.Millisecond)
	}

	matches, err := filepath.Glob(rotatedLogGlob(path))
	if err != nil {
		t.Fatalf("glob failed: %v", err)
	}
	if len(matches) != 2 {
		t.Fatalf("expected 2 backups after pruning, got %d: %v", len(matches), matches)
	}
}

func TestFileRotator_RotationErrorReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat temp dir failed: %v", err)
	}

	r, err := newFileRotator(path, 4, 2)
	if err != nil {
		t.Fatalf("newFileRotator failed: %v", err)
	}
	defer r.Close()

	if _, err := r.Write([]byte("abcd")); err != nil {
		t.Fatalf("initial write failed: %v", err)
	}

	origPerm := info.Mode().Perm()
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("chmod failed: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, origPerm) })

	if _, err := r.Write([]byte("efgh")); err == nil {
		t.Fatalf("expected rotation error when log path is not writable")
	}
}

func TestFileRotator_MaxBackups(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")

	r, err := newFileRotator(path, 4, 1)
	if err != nil {
		t.Fatalf("newFileRotator failed: %v", err)
	}
	defer r.Close()

	for i := 0; i < 4; i++ {
		if _, err := r.Write([]byte("abcd")); err != nil {
			t.Fatalf("write %d failed: %v", i, err)
		}
		time.Sleep(time.Millisecond)
	}

	matches, err := filepath.Glob(rotatedLogGlob(path))
	if err != nil {
		t.Fatalf("glob failed: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 backup with max backups=1, got %d: %v", len(matches), matches)
	}
}

func TestFileRotator_MaxBackupsZeroMeansUnlimited(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")

	r, err := newFileRotator(path, 4, 0)
	if err != nil {
		t.Fatalf("newFileRotator failed: %v", err)
	}
	defer r.Close()

	for i := 0; i < 4; i++ {
		if _, err := r.Write([]byte("abcd")); err != nil {
			t.Fatalf("write %d failed: %v", i, err)
		}
		time.Sleep(time.Millisecond)
	}

	matches, err := filepath.Glob(rotatedLogGlob(path))
	if err != nil {
		t.Fatalf("glob failed: %v", err)
	}
	if len(matches) != 3 {
		t.Fatalf("expected unlimited backups when max backups=0, got %d: %v", len(matches), matches)
	}
}
