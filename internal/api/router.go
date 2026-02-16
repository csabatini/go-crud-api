package api

import (
	"log/slog"
	"net/http"

	"go-storage-api/internal/middleware"
	"go-storage-api/internal/storage"
)

// NewRouter creates a fully wired http.Handler with middleware and routes.
func NewRouter(store storage.Storage, maxUploadSize int64, logger *slog.Logger) http.Handler {
	h := NewHandler(store, maxUploadSize)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/v1/health", h.Health)
	mux.HandleFunc("GET /api/v1/files", h.List)
	mux.HandleFunc("GET /api/v1/files/download", h.Download)
	mux.HandleFunc("POST /api/v1/files/upload", h.Upload)
	mux.HandleFunc("DELETE /api/v1/files", h.Delete)
	mux.HandleFunc("GET /api/v1/files/stat", h.Stat)

	stack := middleware.Chain(
		middleware.RequestID,
		middleware.Logging(logger),
		middleware.PathGuard,
	)

	return stack(mux)
}
