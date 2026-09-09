# Development guide

## Prerequisites

Use Go 1.22 or later, as declared in [go.mod](../go.mod). The application uses
only the standard library. Local content storage requires a writable filesystem
with hard-link support and an available port 8080.

Run all commands from the project directory containing `go.mod`.

## Run locally

```sh
go run ./cmd/server
```

The server creates `data/blobs` relative to the working directory. Keep that
working directory consistent between `go run` and standalone executable runs.

In another terminal:

```sh
curl --fail --show-error http://localhost:8080/healthz
```

The expected response is `{"status":"ok"}`. In Windows PowerShell, use
`curl.exe` to avoid the legacy alias.

Create a small local file named `notes.txt`, then use this PowerShell example to
exercise upload, metadata retrieval, and download:

```powershell
$upload = curl.exe --fail --silent --show-error --form "file=@notes.txt" "http://localhost:8080/files" | ConvertFrom-Json
curl.exe --fail --show-error "http://localhost:8080/files/$($upload.id)?include_checksum=true"
curl.exe --fail --show-error --output downloaded-notes.txt "http://localhost:8080/files/$($upload.id)/content"
```

The multipart request, including overhead, must be smaller than or equal to
1 MiB. Metadata lasts only for the current process; restarting makes earlier IDs
unavailable through the API.

## Build and inspect

```sh
go build ./...
go vet ./...
go test ./...
```

Build a standalone executable when needed:

```sh
go build -o bin/server ./cmd/server
```

On Windows, use `go build -o bin/server.exe ./cmd/server`.

Go documentation comments can be inspected directly:

```sh
go doc ./internal/upload Service.Upload
go doc ./internal/storage/local BlobStore.Put
```

Tests live alongside their packages and include an upload–metadata–download
integration path. When changing behavior, run relevant package tests and the
complete suite before handing off the change. Race detection can additionally
be run with `go test -race ./...` on a supported Go platform with its required C
toolchain.

## Navigating the code

Start with [server startup](../cmd/server/main.go) to see which implementations
are shared, then [routing](../internal/httpapi/router.go) and the relevant handler.

| Concern | Location |
| --- | --- |
| HTTP dependencies | [api.go](../internal/httpapi/api.go) |
| HTTP representations | [response.go](../internal/httpapi/response.go) |
| Upload workflow | [upload service](../internal/upload/service.go) |
| Download workflow | [download service](../internal/download/service.go) |
| Metadata invariants | [metadata.go](../internal/files/metadata.go) |
| In-memory persistence adapter | [metadata store](../internal/storage/memory/metadata_store.go) |
| Filesystem content adapter | [blob store](../internal/storage/local/blob_store.go) |

## Contribution conventions

- Keep request parsing, HTTP status selection, and JSON tags in `httpapi`.
- Keep application workflows independent of concrete storage implementations.
  Define interfaces beside their consumers and select adapters in startup.
- Preserve storage error contracts and use `errors.Is`/`errors.As` to classify
  wrapped errors.
- State who owns streams and dependencies. Close resources at the owning layer.
- Document exported declarations with comments beginning with their names.
  Describe guarantees, preconditions, and failure behavior. Inline comments
  should explain decisions or invariants that are not apparent from the code.
- Format edited Go files with `gofmt`. Keep documentation changes separate from
  behavior changes when practical.

PostgreSQL, recovery, and the REST API redesign remain future implementation work.

## Reference documentation

- [Next-phase lesson](lessons/postgresql-persistence.md): readiness evidence,
  PostgreSQL exercises, recovery todos, and completion checks.
- [Architecture](architecture.md): boundaries, request flows, and adapter contracts.
- [HTTP API](api.md): current routes, response formats, and error mapping.
- [Operations](operations.md): runtime settings, lifecycle, and known limitations.
