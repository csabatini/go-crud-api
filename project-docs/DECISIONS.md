# Architectural Decisions

## Decision Log

### ADR-001: Interface-Based Storage Abstraction

- **Date:** 2026-02-15
- **Status:** Accepted
- **Context:** The API must support multiple file protocols (local filesystem, SMB, FTP, AWS S3) and allow swapping backends without changing HTTP handler code. We need a clean abstraction that decouples protocol-specific logic from the API layer.
- **Decision:** Define a single `storage.Storage` Go interface with five methods (`List`, `Read`, `Write`, `Delete`, `Stat`). Each backend implements this interface in its own package. HTTP handlers accept the interface via dependency injection.
- **Consequences:**
  - Adding a new backend (e.g. SFTP) requires only implementing the interface in a new package and adding a case to the startup switch — no handler changes. The S3 backend validates this: it was added with zero modifications to existing handlers.
  - Testing becomes trivial: mock the interface to test handlers without a real filesystem.
  - Each backend is isolated; SMB dependencies don't affect FTP code.
  - Tradeoff: protocol-specific features (e.g. SMB file locking) cannot be exposed through the generic interface without extending it.

### ADR-002: Streaming I/O via io.Reader / io.ReadCloser

- **Date:** 2026-02-15
- **Status:** Accepted
- **Context:** The service will handle files of arbitrary size. Loading entire files into memory (e.g. `[]byte`) would cause out-of-memory conditions for large files and increase latency.
- **Decision:** The `Storage.Read` method returns `io.ReadCloser` and `Storage.Write` accepts `io.Reader`. File content is streamed from source to destination without full buffering.
- **Consequences:**
  - Memory usage stays constant regardless of file size.
  - Large file transfers (multi-GB) are supported without special handling.
  - Callers must remember to close the `ReadCloser` to avoid resource leaks.
  - Error handling during streaming is more nuanced — partial writes are possible if the stream fails mid-transfer.

### ADR-003: Backend-Per-Package Structure

- **Date:** 2026-02-15
- **Status:** Accepted
- **Context:** Each file protocol (local, SMB, FTP) has different dependencies, connection semantics, and configuration requirements. Mixing them in a single package would create tight coupling and import bloat.
- **Decision:** Each backend lives in its own sub-package under `internal/storage/` (e.g. `internal/storage/local/`, `internal/storage/smb/`, `internal/storage/ftp/`, `internal/storage/s3/`). Each package only imports the libraries it needs.
- **Consequences:**
  - Clear separation of concerns — changes to the FTP backend cannot break the SMB backend.
  - Build dependencies are scoped: if you only use the local backend, SMB/FTP libraries are not compiled in (assuming build tags or selective imports).
  - More packages to navigate, but each is small and focused.

### ADR-004: Configuration via Environment Variables

- **Date:** 2026-02-15
- **Status:** Accepted
- **Context:** The service needs different configuration per environment (development, staging, production) and per backend (local root path vs. SMB host/share vs. FTP credentials). We need a configuration approach that works across container orchestrators, CI/CD, and local development.
- **Decision:** All configuration is loaded from environment variables via `internal/config/`. A `.env` file is supported for local development (never committed). The `STORAGE_BACKEND` variable selects the active backend; backend-specific variables (e.g. `SMB_HOST`, `FTP_PORT`) configure that backend.
- **Consequences:**
  - Follows 12-factor app methodology. Works naturally with Docker, Kubernetes, and CI/CD.
  - No config files to manage or keep in sync across environments.
  - Credentials are never hardcoded or committed to version control.
  - Tradeoff: complex nested configuration is harder to express in flat env vars compared to YAML/TOML.

### ADR-005: Path as Query Parameter

- **Date:** 2026-02-15
- **Status:** Accepted
- **Context:** File paths can contain special characters, deeply nested directories, and characters that conflict with URL path segments (e.g. `/`, `.`, `%`). Encoding file paths as part of the URL path creates ambiguity and routing issues.
- **Decision:** File paths are passed as a `path` query parameter (e.g. `GET /api/v1/files?path=/docs/report.pdf`) rather than embedded in the URL path.
- **Consequences:**
  - No ambiguity between route segments and file path segments.
  - Paths with special characters are handled naturally by standard query parameter encoding.
  - All file endpoints share a consistent parameter convention.
  - Tradeoff: slightly less "RESTful" than path-based resource identification, but more practical for arbitrary filesystem paths.

### ADR-006: Path Traversal Prevention via Middleware

- **Date:** 2026-02-15
- **Status:** Accepted
- **Context:** File path manipulation is the primary attack vector for a file service. Path traversal attacks (e.g. `../../etc/passwd`) could allow access to files outside the intended scope.
- **Decision:** A `pathguard` middleware normalizes all incoming file paths and rejects any path containing `..` or absolute path escapes before the request reaches a handler. Each backend additionally scopes operations to its configured root directory or share.
- **Consequences:**
  - Defense in depth: two layers of protection (middleware + backend scoping).
  - Centralized validation — no need to repeat path checks in every handler.
  - Overly strict normalization could reject legitimate paths in edge cases, but this is a safer default.

### ADR-007: Use internal/ Package Convention

- **Date:** 2026-02-15
- **Status:** Accepted
- **Context:** Go's `internal/` directory convention prevents external packages from importing internal code. Since this is a standalone service (not a library), all application code should be private.
- **Decision:** All application packages live under `internal/`. Only `cmd/server/main.go` sits outside as the entry point.
- **Consequences:**
  - External consumers cannot import our handlers, storage implementations, or config — reducing the API surface we need to maintain.
  - Follows standard Go project layout conventions.
  - If we later need to expose a client SDK, we would create a separate `pkg/` directory for public types.

### ADR-008: AWS S3 Storage Backend

- **Date:** 2026-02-15
- **Status:** Accepted
- **Context:** In addition to filesystem-based protocols (local, SMB, FTP), the service needs to support cloud object storage. AWS S3 is the most widely adopted object storage service and is often required for production deployments where durability, scalability, and availability matter.
- **Decision:** Add an S3 backend (`internal/storage/s3/`) using the AWS SDK for Go v2 (`github.com/aws/aws-sdk-go-v2`). The backend maps file paths to S3 object keys within a configured bucket. It supports the standard AWS credential chain: environment variables (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`), IAM roles, and instance profiles.
- **Consequences:**
  - The service can run on AWS infrastructure without managing credentials manually (via IAM roles).
  - S3 provides 11 nines of durability — suitable for production file storage.
  - The `List` operation maps to `ListObjectsV2` with prefix-based filtering; S3 has no true directory concept, so directory semantics are simulated using `/` delimiters and `CommonPrefixes`.
  - The `Stat` operation maps to `HeadObject`.
  - Streaming is fully supported: `GetObject` returns a streaming body, and `PutObject` accepts an `io.Reader`.
  - Tradeoff: S3 is eventually consistent for certain operations (e.g. listing immediately after a write may not reflect the new object). This is acceptable for this service's use cases.
  - Tradeoff: the AWS SDK is a heavier dependency than the SMB/FTP client libraries, but it is well-maintained and widely used.

### ADR-009: S3 Path-to-Key Mapping

- **Date:** 2026-02-15
- **Status:** Accepted
- **Context:** The `Storage` interface uses filesystem-style paths (e.g. `/docs/report.pdf`), but S3 uses flat object keys with no real directory hierarchy. We need a consistent mapping between the two.
- **Decision:** The S3 backend strips the leading `/` from the file path and prepends an optional configurable prefix (`S3_PREFIX`) to form the object key. For example, with prefix `data/`, path `/docs/report.pdf` becomes key `data/docs/report.pdf`. The `List` operation uses the mapped prefix with `/` as the delimiter to simulate directory listing via `CommonPrefixes`.
- **Consequences:**
  - File paths behave identically regardless of backend — callers don't need to know about S3 key conventions.
  - The optional prefix allows multiple logical filesystems within a single S3 bucket (e.g. per-tenant isolation).
  - `IsDir` in `FileInfo` is inferred from `CommonPrefixes` results rather than a real directory attribute.
  - Empty "directories" (zero-byte keys ending in `/`) are not created; directories exist implicitly when objects exist beneath them.

### ADR-010: Go 1.22 Minimum Version

- **Date:** 2026-02-15
- **Status:** Accepted
- **Context:** The project needs an HTTP router that supports method-based dispatching (e.g. `GET /api/v1/files` vs. `DELETE /api/v1/files` on the same path). Prior to Go 1.22, `net/http.ServeMux` could only match URL paths without method discrimination, requiring either manual method checks in handlers or a third-party router like `chi` or `gorilla/mux`. Go 1.22 introduced enhanced `ServeMux` routing with method patterns, path wildcards, and automatic `405 Method Not Allowed` responses. Additionally, the AWS SDK for Go v2 (`github.com/aws/aws-sdk-go-v2`) requires Go 1.22+ as a minimum version.
- **Decision:** Set `go 1.22` in `go.mod` as the minimum required version. Use the enhanced `net/http.ServeMux` for all routing. Do not introduce a third-party router.
- **Consequences:**
  - The development environment must run Go 1.22 or later (upgrade from 1.18.1 required).
  - Zero external dependencies for the HTTP layer — routing is handled entirely by the standard library.
  - Method-based patterns (`"GET /api/v1/files"`, `"DELETE /api/v1/files"`) enable clean route registration without manual method checks.
  - Automatic `405 Method Not Allowed` for unregistered methods on known paths.
  - Tradeoff: developers must have Go 1.22+ installed. This is a reasonable requirement given Go's rapid adoption of new versions.

### ADR-011: Standard Library HTTP Router (No Third-Party Router)

- **Date:** 2026-02-15
- **Status:** Accepted
- **Context:** The API has 6 routes with distinct literal paths (`/api/v1/files`, `/api/v1/files/download`, `/api/v1/files/upload`, `/api/v1/files/stat`, `/api/v1/health`). All file paths are passed as query parameters (ADR-005), so there are no path parameters to parse from the URL. We evaluated `chi`, `gorilla/mux`, and Go 1.22's enhanced `net/http.ServeMux`.
- **Decision:** Use Go 1.22+ `net/http.ServeMux` exclusively. Route registration looks like: `mux.HandleFunc("GET /api/v1/files", h.List)` and `mux.HandleFunc("DELETE /api/v1/files", h.Delete)`.
- **Consequences:**
  - No external routing dependency. The binary is smaller and there are fewer supply chain risks.
  - The routing topology is simple and immediately understandable to any Go developer.
  - If the API grows to need path parameters, regex matching, or complex middleware per-route, the stdlib mux may become limiting. At that point, migrating to `chi` (which uses the same `http.Handler` interface) would be straightforward.
  - Middleware is applied globally via handler wrapping, not per-route. This is sufficient for our needs (logging, request ID, and path guard all apply to every route).

### ADR-012: Module Path `go-storage-api`

- **Date:** 2026-02-15
- **Status:** Accepted
- **Context:** Go modules require a module path declared in `go.mod`. Convention for publicly hosted modules is to use the repository URL (e.g. `github.com/user/repo`). For private or standalone projects, a simple name suffices.
- **Decision:** Use `go-storage-api` as the module path. Internal imports use this directly (e.g. `go-storage-api/internal/storage`).
- **Consequences:**
  - Simple and concise import paths throughout the codebase.
  - If the module is later published to a public repository, the module path would need to change to include the full repository URL (e.g. `github.com/csabatini/go-storage-api`). This would require updating all internal imports — a breaking change best done before any external consumers exist.
  - For a standalone service that is not imported by other Go modules, a short path is preferable for developer ergonomics.

### ADR-013: Backend Cleanup via io.Closer Type Assertion

- **Date:** 2026-02-15
- **Status:** Accepted
- **Context:** Some storage backends (SMB, FTP) maintain persistent connections that must be closed on shutdown. Others (local, S3) do not require explicit cleanup. Adding a `Close()` method to the `Storage` interface would force all backends to implement it, even when cleanup is unnecessary.
- **Decision:** Do not add `Close()` to the `Storage` interface. Instead, backends that require cleanup implement `io.Closer` in addition to `storage.Storage`. At shutdown, `main.go` checks via type assertion: `if closer, ok := store.(io.Closer); ok { closer.Close() }`.
- **Consequences:**
  - The `Storage` interface remains focused on file operations. Backends are not burdened with no-op `Close()` methods.
  - The cleanup pattern is explicit and visible in `main.go`.
  - New backends that need cleanup simply implement `io.Closer` — no interface changes required.
  - Tradeoff: the cleanup is not enforced by the type system. A backend author could forget to implement `io.Closer`. This is mitigated by code review and documentation.

### ADR-014: FTP Connection Pooling

- **Date:** 2026-02-15
- **Status:** Accepted
- **Context:** The `jlaffaye/ftp` library's `ServerConn` type is not goroutine-safe. A single `ServerConn` cannot be shared across concurrent HTTP requests. Each request needs its own connection, but establishing a new FTP connection per request adds significant latency.
- **Decision:** Implement a channel-based connection pool in the FTP backend. The pool maintains a fixed number of pre-established `ServerConn` instances. Requests acquire a connection from the pool, use it, and return it. If the pool is empty, the request blocks until a connection is available (with a context-based timeout).
- **Consequences:**
  - Concurrent requests are handled safely without connection conflicts.
  - Connection reuse amortizes the cost of FTP authentication across requests.
  - The pool size is configurable, allowing tuning based on expected concurrency and FTP server limits.
  - Stale connections must be detected and replaced (via `conn.NoOp()` health check before use).
  - Tradeoff: adds complexity to the FTP backend compared to the simpler single-connection model used by SMB (whose `Share` type is goroutine-safe).
