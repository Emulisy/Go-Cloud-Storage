# Operations

## Deployment scope

The application is a single-process development service with volatile metadata.
It has no authentication, authorization, TLS listener, rate limiting, storage
quotas, or background reconciliation. The default listener binds all available
interfaces on port 8080; this is not a loopback-only configuration.

Keep the storage directory under the application's control. Key validation
restricts names used by the application, but the adapter is not a sandbox against
another process modifying the directory or its contents.

## Runtime settings

Settings are compile-time constants. There are currently no environment variable,
command-line flag, or configuration-file overrides.

| Setting | Current value | Source |
| --- | --- | --- |
| Listen address | `:8080` | [Server startup](../cmd/server/main.go) |
| Blob directory | `data/blobs`, relative to working directory | [Server startup](../cmd/server/main.go) |
| Header read timeout | 5 seconds | [Server startup](../cmd/server/main.go) |
| Idle keep-alive timeout | 60 seconds | [Server startup](../cmd/server/main.go) |
| Graceful shutdown deadline | 10 seconds | [Server startup](../cmd/server/main.go) |
| Multipart request limit | 1 MiB, including overhead | [Upload handler](../internal/httpapi/upload_file.go) |
| Upload rollback deadline | 5 seconds | [Upload service](../internal/upload/service.go) |

No whole-request read timeout or response write timeout is configured.
`ReadHeaderTimeout` bounds header reading, not the upload body or download
duration.

## Startup and shutdown

Startup creates the metadata store, resolves and creates the blob directory,
constructs the upload/download services, and starts HTTP. Relative storage paths
are resolved from the process working directory, not the executable's location.

Startup or serving failures are logged and produce exit status 1. The
`server listening` message is emitted before the bind attempt; use a successful
health request to establish that the listener is available.

On an interrupt or termination signal handled by the process, the server stops
accepting new connections and allows active requests up to ten seconds to finish.
If graceful shutdown exceeds its deadline or otherwise fails, remaining
connections are closed and the process reports an error. A normal shutdown
returns successfully. Default signal handling is restored after the first signal
so another interrupt can force an exit.

## Storage behavior and limits

### Metadata

Metadata lives only in memory. It is not persisted, reconstructed at startup, or
shared between server processes. Running multiple servers against one blob
directory does not give them shared metadata.

The seeded metadata constructor is intended for controlled fixtures; it copies
supplied values without validation, and later entries with the same ID replace
earlier ones. Normal `Create` operations validate and reject duplicate IDs.

### Blob files

The local adapter requires hard-link support. Temporary files and final keys
reside in the same directory, allowing completed content to be published without
overwriting a competing upload. Filesystems without that capability fail at
publication; there is no fallback that weakens the no-overwrite guarantee.

The adapter requests mode `0700` when creating directories and `0600` for
temporary files. Effective permissions depend on the operating system and
filesystem. Existing directory permissions are not tightened at startup.

Content is synced before publication. The containing directory is not explicitly
synced, and metadata is volatile, so this is not a complete crash-durability
guarantee. Deferred removal of temporary names is best effort. A crash or cleanup
error can leave `.upload-*` files, and failure between blob publication and
metadata creation can leave final-key blobs without metadata.

There is no automatic cleanup or recovery service. Do not treat a missing
metadata record after restart as evidence that its blob can safely be deleted;
the entire in-memory index has been lost. Investigate and preserve needed data
before manual cleanup.

Checksums are computed during upload. Downloads trust saved metadata and do not
detect subsequent external modifications to file content.

### Cancellation

Cancellation is checked at operation boundaries and between upload reads. It
does not forcibly interrupt filesystem calls, mutex waits, or an already blocked
source reader. The rollback deadline similarly depends on backend cooperation.
See [architecture](architecture.md) for the ownership and failure contracts.

## Diagnostics

The standard logger reports startup, shutdown, fatal server errors, and content
streaming failures. There is no access log, request-ID middleware, metrics
endpoint, or distributed tracing. Most pre-response storage errors are mapped to
generic HTTP errors without logging the underlying cause.

| Symptom | Meaning and first check |
| --- | --- |
| Process exits with a bind error | Another process may be using port 8080; inspect listener ownership or change the source setting. |
| Process exits while creating blob storage | Check the working directory, path type, and effective write permissions. |
| Upload receives `413` | Reduce the entire multipart request below the configured limit, including overhead. |
| Upload receives `500` | Check storage capacity, permissions, and hard-link support; the response alone does not identify the backend cause. |
| Earlier IDs return `404` after restart | Expected with the current in-memory metadata store; blobs are not re-indexed. |
| Download receives `500` for known metadata | The blob may be missing or unreadable; inspect the corresponding ID under the blob root. |
| Download is truncated | Check server streaming-error logs and the client connection; the original status may already be `200`. |
| Health succeeds but file operations fail | Health is liveness only and does not validate storage. |

## Persistence milestone

PostgreSQL and recovery work are deferred. Before describing the service as
persistent, implement durable metadata, handle uncertain database commit outcomes,
and define recovery for orphaned blobs and incomplete uploads. Verify an
upload–restart–metadata–download sequence against the same persistent stores.
