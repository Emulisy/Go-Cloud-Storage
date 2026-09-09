# HTTP API reference

This document describes the current implementation. The API is unversioned, and
a REST redesign is deferred. Examples use `http://localhost:8080`.

There is no authentication or authorization. Successful metadata and health
responses use `application/json`. Downloads return binary content. Errors use
`text/plain; charset=utf-8` with a trailing newline, rather than a JSON envelope.

## Routes

| Method | Path | Successful response |
| --- | --- | --- |
| `GET` | `/healthz` | `200 OK`: process liveness |
| `POST` | `/files` | `201 Created`: uploaded file metadata |
| `GET` | `/files/{id}` | `200 OK`: file metadata |
| `GET` | `/files/{id}/content` | `200 OK`: attachment content |

The Go router also matches `HEAD` requests to `GET` routes. There is no dedicated
HEAD implementation. Unsupported methods on known paths receive `405 Method Not
Allowed`; unmatched paths receive `404 Not Found`.

## Health

```sh
curl --fail --show-error http://localhost:8080/healthz
```

```json
{"status":"ok"}
```

This endpoint reports that the HTTP handler is responsive. It does not inspect
disk capacity, permissions, stored content, or metadata consistency.

## Upload a file

Send a `multipart/form-data` request containing a file part named `file`. Let
the client generate the multipart boundary.

```sh
curl --include --form "file=@notes.txt" http://localhost:8080/files
```

On Windows PowerShell, use `curl.exe`.

The handler selects the first file for the `file` field. It does not implement a
batch upload. The entire request is limited to 1,048,576 bytes, including
multipart overhead and additional form fields. Empty file content is accepted;
a non-blank filename is required.

For a five-byte file containing `hello` without a newline, an example response is:

```http
HTTP/1.1 201 Created
Content-Type: application/json
Location: /files/0f8fad5bd9cb469fa16570867728950e
```

```json
{
  "id": "0f8fad5bd9cb469fa16570867728950e",
  "name": "notes.txt",
  "size": 5,
  "checksum": "sha256:2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
}
```

The ID is illustrative; each successful upload receives a newly generated ID.
JSON is shown formatted for readability.

| Status | Body text | Condition |
| --- | --- | --- |
| `400` | `failed to read form file` | Missing file field, malformed multipart input, or an incompatible request format |
| `400` | `invalid upload` | Application rejects the filename or content reader |
| `413` | `request body too large` | Multipart parsing encounters the request body limit |
| `500` | `failed to upload file` | ID generation, storage, or metadata creation fails |

A failed request is not proof that no object was created: the connection may fail
after storage succeeds. Retrying an upload can create an additional object.
There is no idempotency key or duplicate-content detection.

## Retrieve metadata

Replace `FILE_ID` with the ID returned by upload:

```sh
curl --fail --show-error "http://localhost:8080/files/FILE_ID"
curl --fail --show-error "http://localhost:8080/files/FILE_ID?include_checksum=true"
```

| Field | JSON type | Meaning |
| --- | --- | --- |
| `id` | string | Opaque file identifier; generated IDs are 32 lowercase hexadecimal characters |
| `name` | string | Filename obtained from the multipart parser |
| `size` | integer | Number of bytes stored, excluding multipart overhead |
| `checksum` | string | `sha256:` followed by 64 lowercase hexadecimal digits for the local adapter |

The upload response includes the checksum. Metadata responses omit it unless
`include_checksum=true`. An absent, empty, or `false` value omits it; other values,
including `True` and `1`, receive `400`. Additional query parameters are ignored.

| Status | Body text | Condition |
| --- | --- | --- |
| `400` | `invalid include_checksum value` | Unsupported checksum query value |
| `404` | `file not found` | No metadata record exists for the ID |
| `500` | `internal server error` | Metadata lookup fails for another reason |

IDs are looked up as supplied. This endpoint does not enforce the generated-ID
format before lookup.

## Download content

```sh
curl --fail --show-error --output downloaded-notes.txt "http://localhost:8080/files/FILE_ID/content"
```

Successful responses include:

| Header | Value |
| --- | --- |
| `Content-Type` | `application/octet-stream` |
| `Content-Length` | Stored metadata size in bytes |
| `Content-Disposition` | `attachment` with a sanitized filename |
| `X-Content-Type-Options` | `nosniff` |

The download filename removes directory components and control characters and
trims surrounding whitespace. An empty or unusable result falls back to
`download`. This affects the attachment header, not the stored metadata name.

| Status | Body text | Condition |
| --- | --- | --- |
| `400` | `invalid file ID` | The download service rejects a blank ID |
| `404` | `file not found` | Metadata is absent |
| `500` | `internal server error` | Content is missing despite existing metadata, or another storage operation fails |

A streaming failure after response headers are committed cannot change the
status code. Clients may receive incomplete content with an initially successful
status. Downloads do not support range requests, conditional requests, resumable
transfers, or checksum verification on read.

## Other response behavior

The shared JSON writer returns `500` with `failed to encode response` if encoding
fails before headers are sent. Current response types contain only JSON-safe
strings and integers.

There are no list, update, or delete endpoints. Metadata disappears on process
exit, so IDs from earlier runs return `404` even when their blobs remain on disk.
See [operations](operations.md) for lifecycle and persistence details.
