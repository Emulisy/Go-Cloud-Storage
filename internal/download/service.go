// Package download resolves file metadata and opens its stored content.
// Callers own response formatting and the lifetime of returned content streams.
package download

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/Emulisy/Go-Cloud-Storage/internal/blob"
	"github.com/Emulisy/Go-Cloud-Storage/internal/files"
)

var (
	// ErrInvalidID indicates an empty or whitespace-only identifier.
	ErrInvalidID = errors.New("invalid file ID")
	// ErrNotFound indicates that the requested file has no metadata record.
	ErrNotFound = errors.New("file not found")
	// ErrContentUnavailable indicates metadata exists but its blob is missing.
	ErrContentUnavailable = errors.New("file content unavailable")
)

// File contains metadata and an open content stream.
// The caller owns Content and must close it.
type File struct {
	Metadata files.Metadata
	Content  io.ReadCloser
}

// ContentReader opens file bytes and returns a non-nil stream on success.
// The caller owns and must close the stream. Missing content is reported with
// blob.ErrNotFound, directly or wrapped.
type ContentReader interface {
	Open(ctx context.Context, key string) (io.ReadCloser, error)
}

// MetadataReader supplies a stored record by ID. Missing records are reported
// with files.ErrNotFound, directly or wrapped.
type MetadataReader interface {
	Get(ctx context.Context, id string) (files.Metadata, error)
}

// Service coordinates metadata lookup and blob access for downloads.
// It can be shared when its dependencies support concurrent calls.
type Service struct {
	blobs    ContentReader
	metadata MetadataReader
}

// NewService creates a download service from non-nil storage dependencies.
// The caller owns the dependencies and must keep them valid while the service runs.
func NewService(blobs ContentReader, metadata MetadataReader) *Service {
	return &Service{
		blobs:    blobs,
		metadata: metadata,
	}
}

// Download looks up id and opens the content key recorded in its metadata.
// On success, the caller must close the returned File.Content.
//
// Missing metadata and missing content are distinguished by ErrNotFound and
// ErrContentUnavailable. Other storage errors are wrapped with their cause.
// Download does not verify the content's length or checksum against metadata.
func (s *Service) Download(ctx context.Context, id string) (File, error) {
	if err := ctx.Err(); err != nil {
		return File{}, err
	}

	if strings.TrimSpace(id) == "" {
		return File{}, ErrInvalidID
	}

	metadata, err := s.metadata.Get(ctx, id)
	if err != nil {
		if errors.Is(err, files.ErrNotFound) {
			return File{}, ErrNotFound
		}
		return File{}, fmt.Errorf("get metadata: %w", err)
	}

	content, err := s.blobs.Open(ctx, metadata.ID)
	if err != nil {
		if errors.Is(err, blob.ErrNotFound) {
			return File{}, ErrContentUnavailable
		}
		return File{}, fmt.Errorf("open content: %w", err)
	}

	return File{
		Metadata: metadata,
		Content:  content,
	}, nil
}
