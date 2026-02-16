# Implementation Plan

## Context

The project has complete architecture documentation (ARCHITECTURE.md, DECISIONS.md) but no Go source code. This plan implements the full file service API with 4 swappable storage backends (local, SMB, FTP, S3) following the documented architecture.

## Prerequisites

- **Go 1.22+** — Required for stdlib method-based HTTP routing and AWS SDK v2 compatibility (see ADR-010)
- **Module path:** `go-storage-api`

## Implementation Phases

Phases 1-6 use only the Go standard library (zero external dependencies). Phases 7-9 introduce one dependency each for their respective protocol clients.

---

### Phase 1: Foundation

Create the module, storage interface, config loader, and response helpers.

| File | Purpose |
|------|---------|
| `go.mod` | Module `go-storage-api`, Go 1.22 |
| `internal/storage/storage.go` | `Storage` interface, `FileInfo` struct (with JSON tags), `ErrNotFound`/`ErrPermission` sentinel errors |
| `internal/config/config.go` | `Config` struct with nested backend configs. `Load()` reads env vars. Validates backend-specific vars only for the selected backend. |
| `internal/config/config_test.go` | Test defaults, parsing, backend-specific validation using `t.Setenv()` |
| `internal/api/response.go` | `writeJSON`, `writeError` helpers, `ErrorResponse`/`SuccessResponse` types |
| `.env.example` | Replace scaffold content with all backend-specific vars |

**Key details:**
- `FileInfo` uses JSON tags: `json:"name"`, `json:"path"`, `json:"size"`, `json:"isDir"`, `json:"modTime"`
- Config uses `envOrDefault(key, fallback)` and `envRequired(key)` helpers (no external deps like viper)
- `MAX_UPLOAD_SIZE` defaults to `104857600` (100MB), parsed with `strconv.ParseInt`

---

### Phase 2: Middleware

All three middleware components, testable in isolation.

| File | Purpose |
|------|---------|
| `internal/middleware/requestid.go` | Generate UUID via `crypto/rand` (no external dep), set `X-Request-ID` header, store in context. Preserve incoming header if present. Export `RequestIDFromContext()`. |
| `internal/middleware/logging.go` | Wrap `ResponseWriter` to capture status code. Log with `log/slog`: method, path, status, duration, request ID. |
| `internal/middleware/pathguard.go` | Reject `path` query params containing `..` or null bytes. Use `path.Clean()`. Return 400 JSON error. Skip silently if no `path` param. |
| `internal/middleware/middleware.go` | `Chain()` helper to compose middleware |
| `internal/middleware/*_test.go` | Tests for each: traversal variants, status capture, ID generation/preservation |

**Key details:**
- Request ID uses `crypto/rand` to generate UUID v4 — avoids `github.com/google/uuid` dependency
- Logging uses `log/slog` (Go 1.21+ structured logger)
- PathGuard applies globally but is a no-op for routes without a `path` query param

---

### Phase 3: Local Storage Backend

First backend implementation. Enables end-to-end testing with no external dependencies.

| File | Purpose |
|------|---------|
| `internal/storage/local/local.go` | Implements `storage.Storage` against the local filesystem |
| `internal/storage/local/local_test.go` | All 5 interface methods using `t.TempDir()`. Path traversal tests. |

**Key details:**
- Private `safePath()` joins root + requested path, validates result stays under root via `strings.HasPrefix` after `filepath.Clean`
- `List` uses `os.ReadDir` (not deprecated `ioutil.ReadDir`)
- `Read` returns `*os.File` directly (implements `io.ReadCloser`)
- `Write` creates parent dirs with `os.MkdirAll`, then streams via `io.Copy`
- Error mapping: `os.IsNotExist` -> `ErrNotFound`, `os.IsPermission` -> `ErrPermission`

---

### Phase 4: HTTP Handlers, Router, Entry Point

Wire everything together into a running server.

| File | Purpose |
|------|---------|
| `internal/api/handler.go` | `Handler` struct with `storage.Storage` + `maxUploadSize`. Methods: `Health`, `List`, `Download`, `Upload`, `Delete`, `Stat`. |
| `internal/api/router.go` | `NewRouter()` creates `http.ServeMux` with Go 1.22 method patterns. Wraps with middleware. |
| `internal/api/handler_test.go` | Mock storage with function fields per method. Test all handlers via `httptest`. |
| `internal/api/router_test.go` | Verify routing dispatch, 405 for wrong methods, `X-Request-ID` presence. |
| `cmd/server/main.go` | Load config, instantiate local backend, create router, start server with `slog` logging. |

**Key details:**
- Go 1.22 `ServeMux` patterns: `"GET /api/v1/files"`, `"DELETE /api/v1/files"`, etc. (see ADR-011)
- Middleware order: RequestID (outer) -> Logging -> PathGuard (inner)
- Download uses `mime.TypeByExtension` for Content-Type, fallback `application/octet-stream`
- Upload wraps `r.Body` with `http.MaxBytesReader` BEFORE `ParseMultipartForm`
- Error mapping: `ErrNotFound`->404, `ErrPermission`->403, default->500

---

### Phase 5: Integration Tests

| File | Purpose |
|------|---------|
| `tests/integration/local_test.go` | Full lifecycle with `httptest.Server` + local backend |

**Test sequence:** upload -> list -> stat -> download (verify content) -> delete -> verify 404. Path traversal blocked end-to-end.

---

### Phase 6: Dockerfile and Documentation

| File | Purpose |
|------|---------|
| `Dockerfile` | Multi-stage: `golang:1.22-alpine` build, `alpine:3.19` runtime. `CGO_ENABLED=0`. |
| `.gitignore` | Add Go binary name, `data/*` with `!data/.gitkeep` |
| `.dockerignore` | Add `data/` |
| `data/.gitkeep` | Local backend dev directory (tracked, contents ignored) |
| `README.md` | Go prerequisites, quick start, API docs, Docker instructions |
| `project-docs/INFRASTRUCTURE.md` | Environments, env var reference, backend setup guides |

---

### Phase 7: SMB Backend

| File | Purpose |
|------|---------|
| `internal/storage/smb/smb.go` | `storage.Storage` + `io.Closer` via `github.com/hirochachacha/go-smb2` |
| `internal/storage/smb/smb_test.go` | Path validation tests locally. Network tests skipped via `testing.Short()`. |

**Key details:**
- Constructor dials and authenticates, returns `*smb2.Share`
- Implements `io.Closer` for session/share cleanup

---

### Phase 8: FTP Backend

| File | Purpose |
|------|---------|
| `internal/storage/ftp/ftp.go` | `storage.Storage` + `io.Closer` via `github.com/jlaffaye/ftp` |
| `internal/storage/ftp/ftp_test.go` | Path validation tests locally. Network tests skipped via `testing.Short()`. |

**Key details:**
- `ServerConn` is NOT goroutine-safe — requires a channel-based connection pool
- `Stat` implemented via `conn.List(path)` + match (FTP has no native stat)

---

### Phase 9: S3 Backend

| File | Purpose |
|------|---------|
| `internal/storage/s3/s3.go` | `storage.Storage` via `github.com/aws/aws-sdk-go-v2` |
| `internal/storage/s3/s3_test.go` | `toKey` mapping tests. Integration tests behind skip flag. |

**Key details:**
- Private `toKey()` strips leading `/`, prepends configurable prefix (ADR-009)
- `List` uses `ListObjectsV2` with `/` delimiter; `CommonPrefixes` become directories
- `Read` returns `GetObject` response body (`io.ReadCloser`)
- Uses `config.LoadDefaultConfig` for AWS credential chain (env vars, IAM roles, instance profiles)

---

### Phase 10: Final Wiring

| File | Purpose |
|------|---------|
| `cmd/server/main.go` | Add SMB/FTP/S3 switch cases. Graceful shutdown via `signal.NotifyContext`. Backend cleanup via `io.Closer` type assertion. |

---

## Key Implementation Notes

- **Streaming throughout** — `io.Reader`/`io.ReadCloser` for all file content, never `[]byte`
- **Context propagation** — Pass `r.Context()` from handlers to storage methods for cancellation
- **Backend cleanup** — Don't add `Close()` to `Storage` interface; use `if closer, ok := store.(io.Closer); ok { defer closer.Close() }` in main.go
- **Upload size limit** — `http.MaxBytesReader` MUST wrap `r.Body` BEFORE `ParseMultipartForm`
- **Content-Type** — Use `mime.TypeByExtension`, not `http.DetectContentType` (which reads from the stream)
- **Empty path** — Treat `""` and `"/"` equivalently as root directory

## External Dependencies

| Phase | Dependency | Purpose |
|-------|-----------|---------|
| 1-6 | None | Pure Go standard library |
| 7 | `github.com/hirochachacha/go-smb2` | SMB2/3 client |
| 8 | `github.com/jlaffaye/ftp` | FTP client |
| 9 | `github.com/aws/aws-sdk-go-v2`, `/config`, `/service/s3` | AWS S3 client |

## Verification

After each phase:
```bash
go build ./...          # Compiles
go vet ./...            # Static analysis
go test ./...           # All tests pass
```

After Phase 4 (end-to-end):
```bash
# Start server
STORAGE_BACKEND=local LOCAL_ROOT_PATH=./data PORT=8080 go run ./cmd/server

# Test endpoints
curl localhost:8080/api/v1/health
curl -X POST -F "file=@testfile.txt" "localhost:8080/api/v1/files/upload?path=/test.txt"
curl "localhost:8080/api/v1/files?path=/"
curl "localhost:8080/api/v1/files/stat?path=/test.txt"
curl "localhost:8080/api/v1/files/download?path=/test.txt"
curl -X DELETE "localhost:8080/api/v1/files?path=/test.txt"
```
