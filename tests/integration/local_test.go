package integration

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"go-storage-api/internal/api"
	"go-storage-api/internal/storage"
	"go-storage-api/internal/storage/local"
)

// newTestServer creates an httptest.Server backed by a local storage
// rooted in t.TempDir().
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	store, err := local.New(t.TempDir())
	if err != nil {
		t.Fatalf("create local storage: %v", err)
	}

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	router := api.NewRouter(store, 10<<20, logger)
	return httptest.NewServer(router)
}

// uploadFile posts a multipart file upload to the test server.
func uploadFile(t *testing.T, baseURL, path, content string) *http.Response {
	t.Helper()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("file", "upload.bin")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	part.Write([]byte(content))
	w.Close()

	resp, err := http.Post(
		baseURL+"/api/v1/files/upload?path="+path,
		w.FormDataContentType(),
		&buf,
	)
	if err != nil {
		t.Fatalf("upload request: %v", err)
	}
	return resp
}

// --- Health ---

func TestHealth(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/health")
	if err != nil {
		t.Fatalf("health request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body api.SuccessResponse
	json.NewDecoder(resp.Body).Decode(&body)
	if body.Message != "ok" {
		t.Errorf("expected message %q, got %q", "ok", body.Message)
	}
}

// --- Full Lifecycle ---

func TestLifecycle_UploadListStatDownloadDelete(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	filePath := "/docs/test-file.txt"
	fileContent := "hello integration test"

	// 1. Upload
	resp := uploadFile(t, srv.URL, filePath, fileContent)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("upload: expected 201, got %d: %s", resp.StatusCode, body)
	}

	// 2. List parent directory
	resp2, err := http.Get(srv.URL + "/api/v1/files?path=/docs")
	if err != nil {
		t.Fatalf("list request: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", resp2.StatusCode)
	}

	var files []storage.FileInfo
	json.NewDecoder(resp2.Body).Decode(&files)
	if len(files) != 1 {
		t.Fatalf("list: expected 1 file, got %d", len(files))
	}
	if files[0].Name != "test-file.txt" {
		t.Errorf("list: expected name %q, got %q", "test-file.txt", files[0].Name)
	}

	// 3. Stat
	resp3, err := http.Get(srv.URL + "/api/v1/files/stat?path=" + filePath)
	if err != nil {
		t.Fatalf("stat request: %v", err)
	}
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusOK {
		t.Fatalf("stat: expected 200, got %d", resp3.StatusCode)
	}

	var info storage.FileInfo
	json.NewDecoder(resp3.Body).Decode(&info)
	if info.Name != "test-file.txt" {
		t.Errorf("stat: expected name %q, got %q", "test-file.txt", info.Name)
	}
	if info.Size != int64(len(fileContent)) {
		t.Errorf("stat: expected size %d, got %d", len(fileContent), info.Size)
	}
	if info.IsDir {
		t.Error("stat: expected file, got directory")
	}

	// 4. Download and verify content
	resp4, err := http.Get(srv.URL + "/api/v1/files/download?path=" + filePath)
	if err != nil {
		t.Fatalf("download request: %v", err)
	}
	defer resp4.Body.Close()
	if resp4.StatusCode != http.StatusOK {
		t.Fatalf("download: expected 200, got %d", resp4.StatusCode)
	}

	downloaded, err := io.ReadAll(resp4.Body)
	if err != nil {
		t.Fatalf("read download body: %v", err)
	}
	if string(downloaded) != fileContent {
		t.Errorf("download: expected %q, got %q", fileContent, string(downloaded))
	}

	// 5. Delete
	req, err := http.NewRequest(http.MethodDelete, srv.URL+"/api/v1/files?path="+filePath, nil)
	if err != nil {
		t.Fatalf("create delete request: %v", err)
	}
	resp5, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("delete request: %v", err)
	}
	defer resp5.Body.Close()
	if resp5.StatusCode != http.StatusOK {
		t.Fatalf("delete: expected 200, got %d", resp5.StatusCode)
	}

	// 6. Verify file is gone (download returns 404)
	resp6, err := http.Get(srv.URL + "/api/v1/files/download?path=" + filePath)
	if err != nil {
		t.Fatalf("download-after-delete request: %v", err)
	}
	defer resp6.Body.Close()
	if resp6.StatusCode != http.StatusNotFound {
		t.Errorf("download after delete: expected 404, got %d", resp6.StatusCode)
	}
}

// --- Path Traversal ---

func TestPathTraversal_Blocked(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	traversalPaths := []string{
		"/../../../etc/passwd",
		"/..%2F..%2Fetc/passwd",
		"/docs/../../etc/passwd",
	}

	for _, p := range traversalPaths {
		t.Run(p, func(t *testing.T) {
			resp, err := http.Get(srv.URL + "/api/v1/files?path=" + p)
			if err != nil {
				t.Fatalf("request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("expected 400, got %d", resp.StatusCode)
			}
		})
	}
}

func TestPathTraversal_Upload_Blocked(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	resp := uploadFile(t, srv.URL, "/../../../tmp/evil.txt", "malicious")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for traversal upload, got %d", resp.StatusCode)
	}
}

func TestPathTraversal_Download_Blocked(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/files/download?path=/../../../etc/passwd")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for traversal download, got %d", resp.StatusCode)
	}
}

func TestPathTraversal_Delete_Blocked(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	req, err := http.NewRequest(http.MethodDelete, srv.URL+"/api/v1/files?path=/../../../etc/passwd", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for traversal delete, got %d", resp.StatusCode)
	}
}

// --- Request ID ---

func TestRequestID_Present(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/health")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	rid := resp.Header.Get("X-Request-ID")
	if rid == "" {
		t.Error("expected X-Request-ID header, got empty")
	}
}

// --- Method Not Allowed ---

func TestMethodNotAllowed(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	req, err := http.NewRequest(http.MethodPut, srv.URL+"/api/v1/files", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		t.Error("expected non-200 for unsupported method PUT on /api/v1/files")
	}
}

// --- Not Found ---

func TestDownload_NonexistentFile(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/files/download?path=/does-not-exist.txt")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestDelete_NonexistentFile(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	req, err := http.NewRequest(http.MethodDelete, srv.URL+"/api/v1/files?path=/does-not-exist.txt", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}
