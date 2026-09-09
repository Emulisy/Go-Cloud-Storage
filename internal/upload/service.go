// Package upload coordinates creation of file content and metadata.
// Storage contracts are defined here so orchestration remains backend-independent.
package upload

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Emulisy/Go-Cloud-Storage/internal/blob"
	"github.com/Emulisy/Go-Cloud-Storage/internal/files"
)

var (
	// ErrInvalidName indicates an empty or whitespace-only filename.
	ErrInvalidName = errors.New("invalid file name")
	// ErrInvalidContent indicates a nil reader; a reader with zero bytes is valid.
	ErrInvalidContent = errors.New("invalid file content")
)

const rollbackTimeout = 5 * time.Second

// IDGenerator creates a non-empty identifier suitable for the content store.
// Generators used by a shared Service must support concurrent calls.
type IDGenerator func() (string, error)

// ContentStore is the blob behavior required specifically by uploads.
// Put must publish complete content without overwriting an existing key.
// A failed Put must not publish content of its own. Put does not close its source.
type ContentStore interface {
	Put(ctx context.Context, key string, source io.Reader) (blob.PutResult, error)
	Delete(ctx context.Context, key string) error
}

// MetadataWriter is the metadata behavior required by uploads.
// Create must never overwrite an existing record. With the current rollback
// policy, a returned error must mean that no new record was committed.
// A remote database backend needs uncertain outcomes handled before use here.
type MetadataWriter interface {
	Create(ctx context.Context, metadata files.Metadata) error
}

// Service coordinates content storage and metadata storage.
// It holds no per-upload state and can be shared when its dependencies are
// concurrency-safe. It does not own or close those dependencies.
type Service struct {
	blobs    ContentStore
	metadata MetadataWriter
	newID    IDGenerator
}

// NewService creates an upload service using RandomID for new file identifiers.
// Both dependencies must be non-nil and remain valid for the service's lifetime.
func NewService(blobs ContentStore, metadata MetadataWriter) *Service {
	return newService(blobs, metadata, RandomID)
}

func newService(
	blobs ContentStore,
	metadata MetadataWriter,
	newID IDGenerator,
) *Service {
	return &Service{
		blobs:    blobs,
		metadata: metadata,
		newID:    newID,
	}
}

// Upload stores content under a new ID and then creates its metadata record.
// The caller retains ownership of content and must close it when necessary.
// The name is validated for blankness but otherwise preserved.
//
// If metadata creation fails, Upload attempts to delete the newly stored content
// with a separate cleanup deadline. A rollback failure is joined with the
// metadata error. Cancellation is cooperative and depends on the stores.
func (s *Service) Upload(
	ctx context.Context,
	name string,
	content io.Reader,
) (files.Metadata, error) {
	if err := ctx.Err(); err != nil {
		return files.Metadata{}, err
	}

	if strings.TrimSpace(name) == "" {
		return files.Metadata{}, ErrInvalidName
	}
	if content == nil {
		return files.Metadata{}, ErrInvalidContent
	}

	id, err := s.newID()
	if err != nil {
		return files.Metadata{}, fmt.Errorf("generate ID: %w", err)
	}

	result, err := s.blobs.Put(ctx, id, content)
	if err != nil {
		return files.Metadata{}, fmt.Errorf("store content: %w", err)
	}

	metadata := files.Metadata{
		Name:     name,
		ID:       id,
		Size:     result.Size,
		Checksum: result.Checksum,
	}

	if err := s.metadata.Create(ctx, metadata); err != nil {
		createErr := fmt.Errorf("create metadata: %w", err)

		// Cleanup must survive request cancellation but still have a deadline.
		rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), rollbackTimeout)
		defer cancel()

		rollbackErr := s.blobs.Delete(rollbackCtx, id)
		if rollbackErr != nil && !errors.Is(rollbackErr, blob.ErrNotFound) {
			return files.Metadata{}, errors.Join(
				createErr,
				fmt.Errorf("rollback blob: %w", rollbackErr),
			)
		}

		return files.Metadata{}, createErr
	}

	return metadata, nil
}
