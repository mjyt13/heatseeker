package media

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"heatseeker/api/internal/domain"
)

// Local keeps objects as files under a root directory. It cannot presign, so
// the API serves uploads and downloads itself (with signed links).
type Local struct {
	root string
}

// NewLocal creates the root directory if needed.
func NewLocal(root string) (*Local, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("local storage root: %w", err)
	}
	if err := os.MkdirAll(abs, 0o750); err != nil {
		return nil, fmt.Errorf("create local storage root: %w", err)
	}
	return &Local{root: abs}, nil
}

// Kind implements domain.MediaStore.
func (l *Local) Kind() domain.StorageKind { return domain.StorageLocal }

// PresignPut implements domain.MediaStore: not supported.
func (l *Local) PresignPut(context.Context, string, string, time.Duration) (*domain.PresignedRequest, error) {
	return nil, nil
}

// PresignGet implements domain.MediaStore: not supported.
func (l *Local) PresignGet(context.Context, string, string, bool, time.Duration) (*domain.PresignedRequest, error) {
	return nil, nil
}

func (l *Local) path(key string) (string, error) {
	// Cleaning a rooted path resolves every ".." inside the root.
	clean := filepath.Clean("/" + strings.TrimSpace(key))
	if clean == "/" {
		return "", domain.Invalid("key", "bad object key")
	}
	return filepath.Join(l.root, filepath.FromSlash(clean)), nil
}

// Put writes the object atomically.
func (l *Local) Put(_ context.Context, key string, r io.Reader, _ int64, _ string) error {
	dst, err := l.path(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		return fmt.Errorf("local put: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".upload-*")
	if err != nil {
		return fmt.Errorf("local put: %w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := io.Copy(tmp, r); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("local put: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("local put: %w", err)
	}
	if err := os.Rename(tmp.Name(), dst); err != nil {
		return fmt.Errorf("local put: %w", err)
	}
	return nil
}

// Open reads a byte range.
func (l *Local) Open(_ context.Context, key string, offset, length int64) (io.ReadCloser, *domain.ObjectInfo, error) {
	p, err := l.path(key)
	if err != nil {
		return nil, nil, err
	}
	f, err := os.Open(p)
	if err != nil {
		return nil, nil, mapFSErr(err)
	}
	st, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, nil, mapFSErr(err)
	}
	if offset > 0 {
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			_ = f.Close()
			return nil, nil, fmt.Errorf("local open: %w", err)
		}
	}
	var r io.ReadCloser = f
	if length >= 0 {
		r = readCloser{io.LimitReader(f, length), f}
	}
	return r, &domain.ObjectInfo{Key: key, Size: st.Size(), ModTime: st.ModTime()}, nil
}

// Stat returns object metadata.
func (l *Local) Stat(_ context.Context, key string) (*domain.ObjectInfo, error) {
	p, err := l.path(key)
	if err != nil {
		return nil, err
	}
	st, err := os.Stat(p)
	if err != nil {
		return nil, mapFSErr(err)
	}
	return &domain.ObjectInfo{Key: key, Size: st.Size(), ModTime: st.ModTime()}, nil
}

// Move renames an object.
func (l *Local) Move(_ context.Context, src, dst string) error {
	from, err := l.path(src)
	if err != nil {
		return err
	}
	to, err := l.path(dst)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(to), 0o750); err != nil {
		return fmt.Errorf("local move: %w", err)
	}
	if err := os.Rename(from, to); err != nil {
		return mapFSErr(err)
	}
	l.pruneEmpty(filepath.Dir(from))
	return nil
}

// Delete removes an object; a missing object is not an error.
func (l *Local) Delete(_ context.Context, key string) error {
	p, err := l.path(key)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("local delete: %w", err)
	}
	l.pruneEmpty(filepath.Dir(p))
	return nil
}

// pruneEmpty removes empty parent directories up to the root.
func (l *Local) pruneEmpty(dir string) {
	for dir != l.root && strings.HasPrefix(dir, l.root) {
		if err := os.Remove(dir); err != nil {
			return
		}
		dir = filepath.Dir(dir)
	}
}

func mapFSErr(err error) error {
	if errors.Is(err, fs.ErrNotExist) {
		return domain.NotFound("object")
	}
	return fmt.Errorf("local storage: %w", err)
}

type readCloser struct {
	io.Reader
	io.Closer
}
