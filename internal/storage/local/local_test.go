package local

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go-storage-api/internal/storage"
)

func newTestStorage(t *testing.T) *Storage {
	t.Helper()
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	return s
}

// --- List ---

func TestList_EmptyDir(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	files, err := s.List(ctx, "/")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d", len(files))
	}
}

func TestList_WithFiles(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	// Create some files.
	os.WriteFile(filepath.Join(s.root, "a.txt"), []byte("aaa"), 0o644)
	os.Mkdir(filepath.Join(s.root, "subdir"), 0o755)

	files, err := s.List(ctx, "/")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(files))
	}

	names := map[string]bool{}
	for _, f := range files {
		names[f.Name] = true
	}
	if !names["a.txt"] {
		t.Error("missing a.txt")
	}
	if !names["subdir"] {
		t.Error("missing subdir")
	}
}

func TestList_SubDir(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	os.MkdirAll(filepath.Join(s.root, "docs"), 0o755)
	os.WriteFile(filepath.Join(s.root, "docs", "readme.md"), []byte("hi"), 0o644)

	files, err := s.List(ctx, "docs")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].Name != "readme.md" {
		t.Errorf("expected readme.md, got %s", files[0].Name)
	}
	if files[0].Path != "docs/readme.md" {
		t.Errorf("expected path docs/readme.md, got %s", files[0].Path)
	}
}

func TestList_NotFound(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	_, err := s.List(ctx, "nonexistent")
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// --- Read ---

func TestRead_Success(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	content := "hello world"
	os.WriteFile(filepath.Join(s.root, "test.txt"), []byte(content), 0o644)

	rc, err := s.Read(ctx, "test.txt")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(data) != content {
		t.Errorf("expected %q, got %q", content, string(data))
	}
}

func TestRead_NotFound(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	_, err := s.Read(ctx, "missing.txt")
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// --- Write ---

func TestWrite_NewFile(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	content := "new file content"
	err := s.Write(ctx, "output.txt", strings.NewReader(content))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(s.root, "output.txt"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != content {
		t.Errorf("expected %q, got %q", content, string(data))
	}
}

func TestWrite_CreatesParentDirs(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	err := s.Write(ctx, "deep/nested/file.txt", strings.NewReader("deep"))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(s.root, "deep", "nested", "file.txt"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "deep" {
		t.Errorf("expected %q, got %q", "deep", string(data))
	}
}

func TestWrite_Overwrite(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	s.Write(ctx, "file.txt", strings.NewReader("original"))
	s.Write(ctx, "file.txt", strings.NewReader("updated"))

	data, _ := os.ReadFile(filepath.Join(s.root, "file.txt"))
	if string(data) != "updated" {
		t.Errorf("expected %q, got %q", "updated", string(data))
	}
}

// --- Delete ---

func TestDelete_Success(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	os.WriteFile(filepath.Join(s.root, "doomed.txt"), []byte("bye"), 0o644)

	if err := s.Delete(ctx, "doomed.txt"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if _, err := os.Stat(filepath.Join(s.root, "doomed.txt")); !os.IsNotExist(err) {
		t.Error("file should have been deleted")
	}
}

func TestDelete_NotFound(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	err := s.Delete(ctx, "ghost.txt")
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// --- Stat ---

func TestStat_File(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	os.WriteFile(filepath.Join(s.root, "info.txt"), []byte("12345"), 0o644)

	fi, err := s.Stat(ctx, "info.txt")
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if fi.Name != "info.txt" {
		t.Errorf("expected name info.txt, got %s", fi.Name)
	}
	if fi.Size != 5 {
		t.Errorf("expected size 5, got %d", fi.Size)
	}
	if fi.IsDir {
		t.Error("expected IsDir=false")
	}
	if fi.Path != "info.txt" {
		t.Errorf("expected path info.txt, got %s", fi.Path)
	}
}

func TestStat_Dir(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	os.Mkdir(filepath.Join(s.root, "mydir"), 0o755)

	fi, err := s.Stat(ctx, "mydir")
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if !fi.IsDir {
		t.Error("expected IsDir=true")
	}
}

func TestStat_NotFound(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	_, err := s.Stat(ctx, "nope.txt")
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// --- Path traversal ---

func TestSafePath_BlocksTraversal(t *testing.T) {
	s := newTestStorage(t)

	attacks := []string{
		"../etc/passwd",
		"../../etc/shadow",
		"subdir/../../etc/passwd",
		"/../../etc/passwd",
	}

	for _, p := range attacks {
		t.Run(p, func(t *testing.T) {
			_, err := s.safePath(p)
			if !errors.Is(err, storage.ErrPermission) {
				t.Errorf("safePath(%q): expected ErrPermission, got %v", p, err)
			}
		})
	}
}

func TestSafePath_AllowsValid(t *testing.T) {
	s := newTestStorage(t)

	valid := []string{
		"file.txt",
		"docs/readme.md",
		"/docs/readme.md",
		"",
		"/",
	}

	for _, p := range valid {
		t.Run(p, func(t *testing.T) {
			result, err := s.safePath(p)
			if err != nil {
				t.Errorf("safePath(%q): unexpected error: %v", p, err)
			}
			if !strings.HasPrefix(result, s.root) {
				t.Errorf("safePath(%q) = %q, not under root %q", p, result, s.root)
			}
		})
	}
}

// --- Interface compliance ---

var _ storage.Storage = (*Storage)(nil)
