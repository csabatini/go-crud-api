# Architecture

## Overview

A Go web service API for file listing, storage, and retrieval across multiple file protocols. The system uses an interface-based storage abstraction so backends (local filesystem, SMB, FTP, AWS S3) can be swapped without changing HTTP handlers.

The HTTP layer receives a `Storage` interface via dependency injection and delegates all file I/O to it. Backend selection happens once at startup based on environment configuration.

## Components

### 1. HTTP API Layer (`internal/api/`)

REST handlers for file operations. Receives a `Storage` interface, delegates all file I/O to it. Handles multipart uploads, streaming downloads, and JSON responses.

**Routes:**

| Method   | Path                      | Action                 |
|----------|---------------------------|------------------------|
| `GET`    | `/api/v1/files?path=`     | List directory contents|
| `GET`    | `/api/v1/files/download?path=` | Download/retrieve a file |
| `POST`   | `/api/v1/files/upload?path=`   | Upload/store a file    |
| `DELETE` | `/api/v1/files?path=`     | Delete a file          |
| `GET`    | `/api/v1/files/stat?path=`| Get file metadata      |
| `GET`    | `/api/v1/health`          | Health check           |

**Key files:**
- `router.go` — Route registration
- `handler.go` — HTTP handlers (depend on `storage.Storage`)
- `response.go` — Shared JSON response helpers

### 2. Storage Interface (`internal/storage/`)

The contract all backends implement. Contains shared types and sentinel errors.

```go
type FileInfo struct {
    Name    string
    Path    string
    Size    int64
    IsDir   bool
    ModTime time.Time
}

type Storage interface {
    List(ctx context.Context, path string) ([]FileInfo, error)
    Read(ctx context.Context, path string) (io.ReadCloser, error)
    Write(ctx context.Context, path string, r io.Reader) error
    Delete(ctx context.Context, path string) error
    Stat(ctx context.Context, path string) (*FileInfo, error)
}
```

Shared sentinel errors: `ErrNotFound`, `ErrPermission`.

### 3. Storage Backends (`internal/storage/{local,smb,ftp,s3}/`)

Each backend is its own package implementing `storage.Storage`:

- **local** — Uses the `os` package directly. Scoped to a configurable root directory to prevent path traversal.
- **smb** — Uses an SMB2 client library (e.g. `github.com/hirochachacha/go-smb2`). Manages SMB sessions and shares.
- **ftp** — Uses an FTP client library (e.g. `github.com/jlaffaye/ftp`). Manages connection pooling.
- **s3** — Uses the AWS SDK for Go v2 (`github.com/aws/aws-sdk-go-v2`). Maps file paths to S3 object keys within a configured bucket. Supports IAM roles, static credentials, and regional endpoints.

### 4. Configuration (`internal/config/`)

Loads from environment variables (via `.env`). Determines which backend to activate and supplies backend-specific settings (SMB host/share/credentials, FTP host/credentials, local root path, S3 bucket/region/credentials).

### 5. Middleware (`internal/middleware/`)

Cross-cutting concerns applied to all requests:

- `logging.go` — Request logging with method, path, status, duration
- `requestid.go` — Injects a unique request ID header for tracing
- `pathguard.go` — Normalizes and rejects paths containing `..` to prevent traversal attacks

## Data Flow

```
Client Request
    |
    v
[Middleware] --> logging, request ID, path sanitization
    |
    v
[HTTP Handler] --> validates input, parses query params / multipart body
    |
    v
[storage.Storage interface]
    |
    +---> [local.Storage]  --> os.Open / os.Create / os.ReadDir
    +---> [smb.Storage]    --> SMB2 session --> remote share
    +---> [ftp.Storage]    --> FTP connection --> remote server
    +---> [s3.Storage]     --> AWS SDK --> S3 bucket
    |
    v
[HTTP Response] --> JSON metadata or streamed file content
```

### Upload Flow

1. Client sends `POST /api/v1/files/upload?path=/docs/report.pdf` with multipart body
2. Middleware validates the path (no traversal)
3. Handler extracts the file from the multipart form
4. Handler calls `storage.Write(ctx, path, reader)` — file streams directly to backend
5. Handler returns JSON success response

### Download Flow

1. Client sends `GET /api/v1/files/download?path=/docs/report.pdf`
2. Middleware validates the path
3. Handler calls `storage.Read(ctx, path)` — returns `io.ReadCloser`
4. Handler streams content to client with appropriate `Content-Type` header
5. `ReadCloser` is closed after response completes

## Folder Structure

```
go-crud-api/
├── cmd/
│   └── server/
│       └── main.go                  # Entry point: wires config, storage, router
├── internal/
│   ├── api/
│   │   ├── router.go                # Route registration
│   │   ├── handler.go               # HTTP handlers
│   │   └── response.go              # JSON response helpers
│   ├── config/
│   │   └── config.go                # Env-based config loading
│   ├── middleware/
│   │   ├── logging.go               # Request logging
│   │   ├── requestid.go             # Request ID header
│   │   └── pathguard.go             # Path traversal prevention
│   └── storage/
│       ├── storage.go               # Interface + shared types + errors
│       ├── local/
│       │   └── local.go             # Local filesystem backend
│       ├── smb/
│       │   └── smb.go               # SMB protocol backend
│       ├── ftp/
│       │   └── ftp.go               # FTP protocol backend
│       └── s3/
│           └── s3.go                # AWS S3 backend
├── tests/
│   └── integration/                 # Integration tests per backend
├── project-docs/
├── .env.example
├── .gitignore
├── .dockerignore
├── Dockerfile
├── go.mod
└── go.sum
```

## Security Considerations

- **Path traversal** — `pathguard` middleware normalizes and rejects any path containing `..` before it reaches a backend. Each backend also scopes operations to its configured root/share/bucket.
- **Credentials** — SMB/FTP/S3 credentials come from environment variables only, never hardcoded. The S3 backend also supports IAM roles and instance profiles for credential-free deployments on AWS infrastructure.
- **File size limits** — `http.MaxBytesReader` on upload endpoints to prevent out-of-memory conditions.
- **Streaming** — Both upload and download use `io.Reader`/`io.ReadCloser` rather than buffering entire files in memory. The S3 backend uses the SDK's streaming upload/download APIs to maintain this guarantee.

## Wiring (Dependency Injection)

Backend selection happens once at startup in `cmd/server/main.go`:

```go
func main() {
    cfg := config.Load()

    var store storage.Storage
    switch cfg.StorageBackend {
    case "local":
        store = local.New(cfg.Local.RootPath)
    case "smb":
        store = smb.New(cfg.SMB.Host, cfg.SMB.Share, cfg.SMB.User, cfg.SMB.Password)
    case "ftp":
        store = ftp.New(cfg.FTP.Host, cfg.FTP.Port, cfg.FTP.User, cfg.FTP.Password)
    case "s3":
        store = s3.New(cfg.S3.Bucket, cfg.S3.Region, cfg.S3.Prefix)
    default:
        log.Fatalf("unknown storage backend: %s", cfg.StorageBackend)
    }

    router := api.NewRouter(store)
    log.Fatal(http.ListenAndServe(":"+cfg.Port, router))
}
```
