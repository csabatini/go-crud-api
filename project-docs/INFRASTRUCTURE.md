# Infrastructure

## Environments

| Environment | Description | Backend |
|-------------|-------------|---------|
| Development | Local machine, `go run` or binary | `local` with `./data` |
| Docker | Container via `Dockerfile` | `local` with volume mount, or remote backends |
| Production | Container or binary on server | Any backend via env vars |

## Docker

Multi-stage build: `golang:1.22-alpine` (build) -> `alpine:3.19` (runtime). Static binary with `CGO_ENABLED=0`.

```bash
# Build
docker build -t go-storage-api .

# Run with local storage (volume-mounted)
docker run -p 8080:8080 \
  -e STORAGE_BACKEND=local \
  -e LOCAL_ROOT_PATH=/data \
  -v $(pwd)/data:/data \
  go-storage-api

# Run with SMB backend
docker run -p 8080:8080 \
  -e STORAGE_BACKEND=smb \
  -e SMB_HOST=fileserver.local \
  -e SMB_SHARE=shared \
  -e SMB_USER=svc_account \
  -e SMB_PASSWORD=secret \
  go-storage-api
```

## Environment Variables

### Server

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `PORT` | `8080` | No | HTTP listen port |
| `LOG_LEVEL` | `info` | No | `debug`, `info`, `warn`, `error` |
| `STORAGE_BACKEND` | `local` | No | `local`, `smb`, `ftp`, `s3` |
| `MAX_UPLOAD_SIZE` | `104857600` | No | Max upload size in bytes (100MB) |

### Local Backend

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `LOCAL_ROOT_PATH` | `./data` | Yes (if local) | Root directory for file storage |

### SMB Backend

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `SMB_HOST` | — | Yes | SMB server hostname |
| `SMB_PORT` | `445` | No | SMB port |
| `SMB_SHARE` | — | Yes | Share name |
| `SMB_USER` | — | No | Username |
| `SMB_PASSWORD` | — | No | Password |

### FTP Backend

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `FTP_HOST` | — | Yes | FTP server hostname |
| `FTP_PORT` | `21` | No | FTP port |
| `FTP_USER` | — | No | Username |
| `FTP_PASSWORD` | — | No | Password |

### S3 Backend

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `S3_BUCKET` | — | Yes | S3 bucket name |
| `S3_REGION` | `us-east-1` | No | AWS region |
| `S3_PREFIX` | — | No | Key prefix for all objects |
| `AWS_ACCESS_KEY_ID` | — | No | Static credential (or use IAM roles) |
| `AWS_SECRET_ACCESS_KEY` | — | No | Static credential (or use IAM roles) |

## Backend Setup Guides

### Local

No setup required. The server creates `LOCAL_ROOT_PATH` on startup if it doesn't exist.

### SMB

1. Ensure the SMB share is accessible from the server
2. Set `SMB_HOST`, `SMB_SHARE`, and credentials in env vars
3. The SMB client connects on startup and keeps the session open

### FTP

1. Ensure the FTP server accepts connections from the server
2. Set `FTP_HOST` and credentials in env vars
3. Connection pooling manages multiple concurrent requests

### S3

1. Create an S3 bucket in your target region
2. Set `S3_BUCKET` and `S3_REGION`
3. Credentials via env vars (`AWS_ACCESS_KEY_ID`/`AWS_SECRET_ACCESS_KEY`) or IAM roles
4. Optional: set `S3_PREFIX` to scope all objects under a key prefix
