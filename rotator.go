package logx

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// fileRotator is a simple size-based log rotator.
type fileRotator struct {
	path    string
	mu      sync.Mutex
	f       *os.File
	maxSize int
	backups int
	size    int64
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

	info, _ := f.Stat()
	r := &fileRotator{path: path, f: f, maxSize: maxSize, backups: backups, size: info.Size()}
	return r, nil
}

func (r *fileRotator) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.maxSize > 0 && r.size+int64(len(p)) > int64(r.maxSize) {
		if err := r.rotate(); err != nil {
			// if rotation fails, still attempt to write to current file
		}
	}

	n, err := r.f.Write(p)
	r.size += int64(n)
	return n, err
}

func (r *fileRotator) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.f != nil {
		err := r.f.Close()
		r.f = nil
		return err
	}
	return nil
}

func (r *fileRotator) rotate() error {
	if r.f != nil {
		r.f.Close()
	}

	ts := time.Now().UTC().Format("20060102T150405.000000000")
	rotated := fmt.Sprintf("%s.%s", r.path, ts)
	if err := os.Rename(r.path, rotated); err != nil {
		// if rename fails, try to reopen existing file
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
		return err
	}
	r.f = f
	r.size = 0

	if r.backups > 0 {
		// remove older backups
		dir := filepath.Dir(r.path)
		base := filepath.Base(r.path)
		entries, _ := filepath.Glob(filepath.Join(dir, base+".*"))
		sort.Strings(entries)
		if len(entries) > r.backups {
			remove := entries[:len(entries)-r.backups]
			for _, p := range remove {
				_ = os.Remove(p)
			}
		}
	}

	return nil
}

// Ensure fileRotator implements io.WriteCloser
var _ io.WriteCloser = (*fileRotator)(nil)
