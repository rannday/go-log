package logx

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const rotatedLogSuffix = ".logx-rotated-"

// fileRotator is a simple size-based log rotator.
// It is intentionally minimal: no time rotation, compression, or external deps.
type fileRotator struct {
	path    string
	mu      sync.Mutex
	f       *os.File
	maxSize int
	backups int
	size    int64
	closed  bool
}

func newFileRotator(path string, maxSize int, backups int) (*fileRotator, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}

	info, err := f.Stat()
	var size int64
	if err == nil {
		size = info.Size()
	}
	r := &fileRotator{path: path, f: f, maxSize: maxSize, backups: backups, size: size}
	return r, nil
}

func (r *fileRotator) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return 0, errors.New("file rotator closed")
	}

	if r.maxSize > 0 && r.size > 0 && r.size+int64(len(p)) > int64(r.maxSize) {
		if err := r.rotateLocked(); err != nil {
			return 0, err
		}
	}

	if r.f == nil {
		return 0, errors.New("file rotator closed")
	}

	n, err := r.f.Write(p)
	r.size += int64(n)
	if err != nil {
		return n, err
	}

	if r.maxSize > 0 && r.size > int64(r.maxSize) {
		if rotErr := r.rotateLocked(); rotErr != nil {
			return n, rotErr
		}
	}

	return n, nil
}

func (r *fileRotator) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	if r.f != nil {
		err := r.f.Close()
		r.f = nil
		return err
	}
	return nil
}

func (r *fileRotator) rotateLocked() error {
	if r.f != nil {
		_ = r.f.Close()
	}

	ts := time.Now().UTC().Format("20060102T150405.000000000")
	rotated := r.path + rotatedLogSuffix + ts
	if err := os.Rename(r.path, rotated); err != nil {
		f, err2 := os.OpenFile(r.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err2 != nil {
			return err2
		}
		r.f = f
		info, _ := f.Stat()
		r.size = info.Size()
		return err
	}

	f, err := os.OpenFile(r.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		_ = os.Rename(rotated, r.path)
		fallback, reopenErr := os.OpenFile(r.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if reopenErr != nil {
			return err
		}
		r.f = fallback
		info, _ := fallback.Stat()
		r.size = info.Size()
		return err
	}
	r.f = f
	r.size = 0

	if r.backups > 0 {
		entries, _ := filepath.Glob(r.path + rotatedLogSuffix + "*")
		sort.Strings(entries)
		for len(entries) > r.backups {
			_ = os.Remove(entries[0])
			entries = entries[1:]
		}
	}

	return nil
}

func rotatedLogGlob(path string) string {
	if strings.HasSuffix(path, rotatedLogSuffix) {
		return path + "*"
	}
	return path + rotatedLogSuffix + "*"
}

// Ensure fileRotator implements io.WriteCloser
var _ io.WriteCloser = (*fileRotator)(nil)