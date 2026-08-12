package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Blobs is the narrow slice of object storage this app actually needs: put a
// photographed handout somewhere, read it back, drop it. The presigned-URL
// interface above is a wider contract than that, and nothing implements it yet.
type Blobs interface {
	Put(ctx context.Context, key string, data []byte, contentType string) error
	Get(ctx context.Context, key string) ([]byte, error)
	Delete(ctx context.Context, key string) error
}

// FileBlobs keeps objects on the local disk. It is the default because an app
// that cannot show you the handout it read is missing the obvious next question
// — "which print was that?" — and waiting on a MinIO deployment to answer it is
// a poor trade. Swap it for object storage when there is more than one server.
type FileBlobs struct{ root string }

func NewFileBlobs(root string) (*FileBlobs, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("create blob directory: %w", err)
	}
	return &FileBlobs{root: root}, nil
}

// path refuses anything that could climb out of the root. Keys are generated
// internally today, but that is exactly the kind of assumption that stops being
// true later.
func (f *FileBlobs) path(key string) (string, error) {
	clean := filepath.Clean("/" + key)
	if strings.Contains(clean, "..") {
		return "", fmt.Errorf("invalid object key %q", key)
	}
	return filepath.Join(f.root, clean), nil
}

func (f *FileBlobs) Put(ctx context.Context, key string, data []byte, contentType string) error {
	p, err := f.path(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}

func (f *FileBlobs) Get(ctx context.Context, key string) ([]byte, error) {
	p, err := f.path(key)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return nil, ErrNoSuchObject
	}
	return data, err
}

func (f *FileBlobs) Delete(ctx context.Context, key string) error {
	p, err := f.path(key)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

var ErrNoSuchObject = fmt.Errorf("no such object")

// discard throws everything away. It stands in when no storage is configured,
// so the analysis path still works and only the "look at it again" part is
// missing.
type discard struct{}

func NewDiscardBlobs() Blobs                                      { return discard{} }
func (discard) Put(context.Context, string, []byte, string) error { return nil }
func (discard) Get(context.Context, string) ([]byte, error)       { return nil, ErrNoSuchObject }
func (discard) Delete(context.Context, string) error              { return nil }
