package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go-storage-api/internal/storage"
)

// mockStorage implements storage.Storage with function fields for per-test control.
type mockStorage struct {
	listFn   func(ctx context.Context, path string) ([]storage.FileInfo, error)
	readFn   func(ctx context.Context, path string) (io.ReadCloser, error)
	writeFn  func(ctx context.Context, path string, r io.Reader) error
	deleteFn func(ctx context.Context, path string) error
	statFn   func(ctx context.Context, path string) (*storage.FileInfo, error)
}

func (m *mockStorage) List(ctx context.Context, path string) ([]storage.FileInfo, error) {
	return m.listFn(ctx, path)
}
func (m *mockStorage) Read(ctx context.Context, path string) (io.ReadCloser, error) {
	return m.readFn(ctx, path)
}
func (m *mockStorage) Write(ctx context.Context, path string, r io.Reader) error {
	return m.writeFn(ctx, path, r)
}
func (m *mockStorage) Delete(ctx context.Context, path string) error {
	return m.deleteFn(ctx, path)
}
func (m *mockStorage) Stat(ctx context.Context, path string) (*storage.FileInfo, error) {
	return m.statFn(ctx, path)
}

func newTestHandler(store *mockStorage) *Handler {
	return NewHandler(store, 10<<20) // 10MB
}

// --- Health ---

func TestHealth(t *testing.T) {
	h := newTestHandler(&mockStorage{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rr := httptest.NewRecorder()

	h.Health(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	var body SuccessResponse
	json.NewDecoder(rr.Body).Decode(&body)
	if body.Message != "ok" {
		t.Errorf("expected message %q, got %q", "ok", body.Message)
	}
}

// --- List ---

func TestList_Success(t *testing.T) {
	store := &mockStorage{
		listFn: func(_ context.Context, path string) ([]storage.FileInfo, error) {
			return []storage.FileInfo{
				{Name: "file.txt", Path: "file.txt", Size: 100, ModTime: time.Now()},
			}, nil
		},
	}
	h := newTestHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/files?path=/", nil)
	rr := httptest.NewRecorder()
	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	var files []storage.FileInfo
	json.NewDecoder(rr.Body).Decode(&files)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].Name != "file.txt" {
		t.Errorf("expected file.txt, got %s", files[0].Name)
	}
}

func TestList_DefaultsToRoot(t *testing.T) {
	var capturedPath string
	store := &mockStorage{
		listFn: func(_ context.Context, path string) ([]storage.FileInfo, error) {
			capturedPath = path
			return nil, nil
		},
	}
	h := newTestHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/files", nil)
	rr := httptest.NewRecorder()
	h.List(rr, req)

	if capturedPath != "/" {
		t.Errorf("expected path %q, got %q", "/", capturedPath)
	}
}

func TestList_NotFound(t *testing.T) {
	store := &mockStorage{
		listFn: func(_ context.Context, _ string) ([]storage.FileInfo, error) {
			return nil, storage.ErrNotFound
		},
	}
	h := newTestHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/files?path=/nope", nil)
	rr := httptest.NewRecorder()
	h.List(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

// --- Download ---

func TestDownload_Success(t *testing.T) {
	content := "file contents here"
	store := &mockStorage{
		readFn: func(_ context.Context, _ string) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(content)), nil
		},
	}
	h := newTestHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/files/download?path=readme.txt", nil)
	rr := httptest.NewRecorder()
	h.Download(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if rr.Body.String() != content {
		t.Errorf("expected body %q, got %q", content, rr.Body.String())
	}
	ct := rr.Header().Get("Content-Type")
	if ct != "text/plain; charset=utf-8" {
		t.Errorf("expected text/plain content-type, got %q", ct)
	}
}

func TestDownload_UnknownExtension(t *testing.T) {
	store := &mockStorage{
		readFn: func(_ context.Context, _ string) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("binary")), nil
		},
	}
	h := newTestHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/files/download?path=data.xyz123", nil)
	rr := httptest.NewRecorder()
	h.Download(rr, req)

	ct := rr.Header().Get("Content-Type")
	if ct != "application/octet-stream" {
		t.Errorf("expected application/octet-stream, got %q", ct)
	}
}

func TestDownload_MissingPath(t *testing.T) {
	h := newTestHandler(&mockStorage{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/files/download", nil)
	rr := httptest.NewRecorder()
	h.Download(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestDownload_NotFound(t *testing.T) {
	store := &mockStorage{
		readFn: func(_ context.Context, _ string) (io.ReadCloser, error) {
			return nil, storage.ErrNotFound
		},
	}
	h := newTestHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/files/download?path=gone.txt", nil)
	rr := httptest.NewRecorder()
	h.Download(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

// --- Upload ---

func createMultipartRequest(t *testing.T, path, filename, content string) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	part.Write([]byte(content))
	w.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/files/upload?path="+path, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

func TestUpload_Success(t *testing.T) {
	var writtenContent string
	store := &mockStorage{
		writeFn: func(_ context.Context, _ string, r io.Reader) error {
			data, _ := io.ReadAll(r)
			writtenContent = string(data)
			return nil
		},
	}
	h := newTestHandler(store)

	req := createMultipartRequest(t, "upload.txt", "upload.txt", "uploaded data")
	rr := httptest.NewRecorder()
	h.Upload(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", rr.Code)
	}
	if writtenContent != "uploaded data" {
		t.Errorf("expected written content %q, got %q", "uploaded data", writtenContent)
	}
}

func TestUpload_MissingPath(t *testing.T) {
	h := newTestHandler(&mockStorage{})

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	w.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/files/upload", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rr := httptest.NewRecorder()
	h.Upload(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestUpload_MissingFile(t *testing.T) {
	h := newTestHandler(&mockStorage{})

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	w.WriteField("other", "value")
	w.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/files/upload?path=test.txt", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rr := httptest.NewRecorder()
	h.Upload(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

// --- Delete ---

func TestDelete_Success(t *testing.T) {
	store := &mockStorage{
		deleteFn: func(_ context.Context, _ string) error {
			return nil
		},
	}
	h := newTestHandler(store)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/files?path=trash.txt", nil)
	rr := httptest.NewRecorder()
	h.Delete(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestDelete_MissingPath(t *testing.T) {
	h := newTestHandler(&mockStorage{})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/files", nil)
	rr := httptest.NewRecorder()
	h.Delete(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestDelete_NotFound(t *testing.T) {
	store := &mockStorage{
		deleteFn: func(_ context.Context, _ string) error {
			return storage.ErrNotFound
		},
	}
	h := newTestHandler(store)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/files?path=gone.txt", nil)
	rr := httptest.NewRecorder()
	h.Delete(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

// --- Stat ---

func TestStat_Success(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	store := &mockStorage{
		statFn: func(_ context.Context, _ string) (*storage.FileInfo, error) {
			return &storage.FileInfo{
				Name:    "info.txt",
				Path:    "info.txt",
				Size:    42,
				IsDir:   false,
				ModTime: now,
			}, nil
		},
	}
	h := newTestHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/files/stat?path=info.txt", nil)
	rr := httptest.NewRecorder()
	h.Stat(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	var info storage.FileInfo
	json.NewDecoder(rr.Body).Decode(&info)
	if info.Name != "info.txt" {
		t.Errorf("expected name info.txt, got %s", info.Name)
	}
	if info.Size != 42 {
		t.Errorf("expected size 42, got %d", info.Size)
	}
}

func TestStat_MissingPath(t *testing.T) {
	h := newTestHandler(&mockStorage{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/files/stat", nil)
	rr := httptest.NewRecorder()
	h.Stat(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestStat_PermissionDenied(t *testing.T) {
	store := &mockStorage{
		statFn: func(_ context.Context, _ string) (*storage.FileInfo, error) {
			return nil, storage.ErrPermission
		},
	}
	h := newTestHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/files/stat?path=secret.txt", nil)
	rr := httptest.NewRecorder()
	h.Stat(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}
