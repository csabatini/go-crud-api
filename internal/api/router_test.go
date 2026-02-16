package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go-storage-api/internal/storage"
)

func newTestRouter() http.Handler {
	store := &mockStorage{
		listFn: func(_ context.Context, _ string) ([]storage.FileInfo, error) {
			return []storage.FileInfo{}, nil
		},
		readFn: func(_ context.Context, _ string) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("data")), nil
		},
		writeFn: func(_ context.Context, _ string, _ io.Reader) error {
			return nil
		},
		deleteFn: func(_ context.Context, _ string) error {
			return nil
		},
		statFn: func(_ context.Context, _ string) (*storage.FileInfo, error) {
			return &storage.FileInfo{Name: "test"}, nil
		},
	}

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	return NewRouter(store, 10<<20, logger)
}

func TestRouter_HealthRoute(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	var body SuccessResponse
	json.NewDecoder(rr.Body).Decode(&body)
	if body.Message != "ok" {
		t.Errorf("expected message %q, got %q", "ok", body.Message)
	}
}

func TestRouter_ListRoute(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/files", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestRouter_StatRoute(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/files/stat?path=test", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestRouter_DownloadRoute(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/files/download?path=test.txt", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestRouter_DeleteRoute(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/files?path=test.txt", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestRouter_WrongMethod(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/files", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestRouter_RequestIDHeader(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	xrid := rr.Header().Get("X-Request-ID")
	if xrid == "" {
		t.Error("expected X-Request-ID header in response")
	}
}

func TestRouter_NotFoundRoute(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/nonexistent", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}
