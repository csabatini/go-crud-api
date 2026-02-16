# go-storage-api

A Go web service API for file listing, storage, and retrieval across multiple file protocols. The service uses an interface-based storage abstraction so backends can be swapped without changing application code.

## Supported Storage Backends

- **Local** — Unix filesystem scoped to a configurable root directory
- **SMB** — SMB2/3 protocol for Windows/Samba file shares
- **FTP** — FTP protocol with connection pooling
- **S3** — AWS S3 with IAM role and static credential support

## Prerequisites

- Go 1.22+

## Getting Started

1. Copy `.env.example` to `.env` and fill in your values
2. Build and run the project:

```bash
go build -o go-storage-api ./cmd/server
STORAGE_BACKEND=local LOCAL_ROOT_PATH=./data PORT=8080 ./go-storage-api
```

Or run directly:

```bash
STORAGE_BACKEND=local LOCAL_ROOT_PATH=./data PORT=8080 go run ./cmd/server
```

## API Endpoints

| Method   | Path                           | Action                 |
|----------|--------------------------------|------------------------|
| `GET`    | `/api/v1/files?path=`          | List directory contents|
| `GET`    | `/api/v1/files/download?path=` | Download a file        |
| `POST`   | `/api/v1/files/upload?path=`   | Upload a file          |
| `DELETE` | `/api/v1/files?path=`          | Delete a file          |
| `GET`    | `/api/v1/files/stat?path=`     | Get file metadata      |
| `GET`    | `/api/v1/health`               | Health check           |

## Configuration

The active storage backend is selected via the `STORAGE_BACKEND` environment variable. Only the variables for the selected backend are required.

See `.env.example` for the full list of environment variables.

## Project Structure

```
go-storage-api/
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
│   ├── ARCHITECTURE.md              # System overview and data flow
│   ├── DECISIONS.md                 # Architectural decision records
│   └── INFRASTRUCTURE.md            # Deployment and environment details
├── data/                            # Local backend dev storage (contents gitignored)
├── .env.example                     # Environment variable template
├── Dockerfile
├── go.mod
└── go.sum
```

## Documentation

| Document | Purpose |
|----------|---------|
| `PLAN.md` | Implementation plan and phasing |
| `project-docs/ARCHITECTURE.md` | System architecture, data flow, security |
| `project-docs/DECISIONS.md` | Architectural decision records (ADR-001 through ADR-014) |
| `project-docs/INFRASTRUCTURE.md` | Deployment and environment configuration |
