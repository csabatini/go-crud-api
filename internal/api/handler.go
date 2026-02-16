package api

import (
	"errors"
	"io"
	"mime"
	"net/http"
	"path/filepath"

	"go-storage-api/internal/storage"
)

// Handler holds dependencies for HTTP handlers.
type Handler struct {
	store         storage.Storage
	maxUploadSize int64
}

// NewHandler creates a Handler with the given storage backend and upload limit.
func NewHandler(store storage.Storage, maxUploadSize int64) *Handler {
	return &Handler{store: store, maxUploadSize: maxUploadSize}
}

// Health returns a simple health check response.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, SuccessResponse{Message: "ok"})
}

// List returns the contents of a directory.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Query().Get("path")
	if p == "" {
		p = "/"
	}

	files, err := h.store.List(r.Context(), p)
	if err != nil {
		handleStorageError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, files)
}

// Download streams a file to the client.
func (h *Handler) Download(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Query().Get("path")
	if p == "" {
		writeError(w, http.StatusBadRequest, "path query parameter is required")
		return
	}

	rc, err := h.store.Read(r.Context(), p)
	if err != nil {
		handleStorageError(w, err)
		return
	}
	defer rc.Close()

	ct := mime.TypeByExtension(filepath.Ext(p))
	if ct == "" {
		ct = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ct)

	io.Copy(w, rc)
}

// Upload receives a multipart file and writes it to storage.
func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Query().Get("path")
	if p == "" {
		writeError(w, http.StatusBadRequest, "path query parameter is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, h.maxUploadSize)

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart form: "+err.Error())
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file field is required: "+err.Error())
		return
	}
	defer file.Close()

	if err := h.store.Write(r.Context(), p, file); err != nil {
		handleStorageError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, SuccessResponse{Message: "file uploaded"})
}

// Delete removes a file from storage.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Query().Get("path")
	if p == "" {
		writeError(w, http.StatusBadRequest, "path query parameter is required")
		return
	}

	if err := h.store.Delete(r.Context(), p); err != nil {
		handleStorageError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, SuccessResponse{Message: "file deleted"})
}

// Stat returns metadata for a file or directory.
func (h *Handler) Stat(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Query().Get("path")
	if p == "" {
		writeError(w, http.StatusBadRequest, "path query parameter is required")
		return
	}

	info, err := h.store.Stat(r.Context(), p)
	if err != nil {
		handleStorageError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, info)
}

// handleStorageError maps storage sentinel errors to HTTP status codes.
func handleStorageError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, storage.ErrNotFound):
		writeError(w, http.StatusNotFound, "not found")
	case errors.Is(err, storage.ErrPermission):
		writeError(w, http.StatusForbidden, "permission denied")
	default:
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}
