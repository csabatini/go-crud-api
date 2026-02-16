package local

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go-storage-api/internal/storage"
)

// Storage implements storage.Storage against the local filesystem.
type Storage struct {
	root string
}

// New creates a local storage backend rooted at the given directory.
func New(root string) (*Storage, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve root path: %w", err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, fmt.Errorf("create root directory: %w", err)
	}
	return &Storage{root: abs}, nil
}

func (s *Storage) List(_ context.Context, path string) ([]storage.FileInfo, error) {
	full, err := s.safePath(path)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(full)
	if err != nil {
		return nil, mapError(err)
	}

	files := make([]storage.FileInfo, 0, len(entries))
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			return nil, mapError(err)
		}
		rel, _ := filepath.Rel(s.root, filepath.Join(full, e.Name()))
		files = append(files, storage.FileInfo{
			Name:    e.Name(),
			Path:    filepath.ToSlash(rel),
			Size:    info.Size(),
			IsDir:   e.IsDir(),
			ModTime: info.ModTime(),
		})
	}
	return files, nil
}

func (s *Storage) Read(_ context.Context, path string) (io.ReadCloser, error) {
	full, err := s.safePath(path)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(full)
	if err != nil {
		return nil, mapError(err)
	}
	return f, nil
}

func (s *Storage) Write(_ context.Context, path string, r io.Reader) error {
	full, err := s.safePath(path)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return mapError(err)
	}

	f, err := os.Create(full)
	if err != nil {
		return mapError(err)
	}
	defer f.Close()

	if _, err := io.Copy(f, r); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}

func (s *Storage) Delete(_ context.Context, path string) error {
	full, err := s.safePath(path)
	if err != nil {
		return err
	}

	if err := os.Remove(full); err != nil {
		return mapError(err)
	}
	return nil
}

func (s *Storage) Stat(_ context.Context, path string) (*storage.FileInfo, error) {
	full, err := s.safePath(path)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(full)
	if err != nil {
		return nil, mapError(err)
	}

	rel, _ := filepath.Rel(s.root, full)
	return &storage.FileInfo{
		Name:    info.Name(),
		Path:    filepath.ToSlash(rel),
		Size:    info.Size(),
		IsDir:   info.IsDir(),
		ModTime: info.ModTime(),
	}, nil
}

// safePath resolves the requested path against the root directory and ensures
// the result stays within root to prevent directory traversal.
func (s *Storage) safePath(requested string) (string, error) {
	// Treat empty or "/" as the root directory.
	if requested == "" || requested == "/" {
		return s.root, nil
	}

	joined := filepath.Join(s.root, filepath.FromSlash(requested))
	cleaned := filepath.Clean(joined)

	if !strings.HasPrefix(cleaned, s.root) {
		return "", storage.ErrPermission
	}
	return cleaned, nil
}

// mapError converts os-level errors to storage sentinel errors.
func mapError(err error) error {
	if os.IsNotExist(err) {
		return storage.ErrNotFound
	}
	if os.IsPermission(err) {
		return storage.ErrPermission
	}
	return err
}
