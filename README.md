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
2. Build and run:

```bash
go build -o server ./cmd/server
STORAGE_BACKEND=local LOCAL_ROOT_PATH=./data PORT=8080 ./server
```

Or run directly:

```bash
STORAGE_BACKEND=local LOCAL_ROOT_PATH=./data PORT=8080 go run ./cmd/server
```

### Docker

Build and run with Docker:

```bash
docker build -t go-storage-api .
docker run -p 8080:8080 \
  -e STORAGE_BACKEND=local \
  -e LOCAL_ROOT_PATH=/data \
  -v $(pwd)/data:/data \
  go-storage-api
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

## API Usage

```bash
# Health check
curl localhost:8080/api/v1/health

# Upload a file
curl -X POST -F "file=@report.pdf" "localhost:8080/api/v1/files/upload?path=/docs/report.pdf"

# List directory
curl "localhost:8080/api/v1/files?path=/docs"

# File metadata
curl "localhost:8080/api/v1/files/stat?path=/docs/report.pdf"

# Download a file
curl -o report.pdf "localhost:8080/api/v1/files/download?path=/docs/report.pdf"

# Delete a file
curl -X DELETE "localhost:8080/api/v1/files?path=/docs/report.pdf"
```

## Configuration

The active storage backend is selected via the `STORAGE_BACKEND` environment variable. Only the variables for the selected backend are required.

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | Server listen port |
| `LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `STORAGE_BACKEND` | `local` | Backend: `local`, `smb`, `ftp`, `s3` |
| `MAX_UPLOAD_SIZE` | `104857600` | Max upload size in bytes (default 100MB) |
| `LOCAL_ROOT_PATH` | `./data` | Root directory for local backend |

See `.env.example` for the full list including SMB, FTP, and S3 variables.

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
