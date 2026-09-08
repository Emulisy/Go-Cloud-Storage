package upload

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
	ErrInvalidName    = errors.New("invalid file name")
	ErrInvalidContent = errors.New("invalid file content")
)

type IDGenerator func() (string, error)

// Service coordinates content storage and metadata storage.
type Service struct {
	blobs    blob.Store
	metadata files.Writer
	newID    IDGenerator
}

func NewService(blobs blob.Store, metadata files.Writer) *Service {
	return newService(blobs, metadata, RandomID)
}

func newService(
	blobs blob.Store,
	metadata files.Writer,
	newID IDGenerator,
) *Service {
	return &Service{
		blobs:    blobs,
		metadata: metadata,
		newID:    newID,
	}
}

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

	res, err := s.blobs.Put(ctx, id, content)
	if err != nil {
		return files.Metadata{}, fmt.Errorf("store content: %w", err)
	}

	metadata := files.Metadata{
		Name:     name,
		ID:       id,
		Size:     res.Size,
		Checksum: res.Checksum,
	}

	if err := s.metadata.Create(ctx, metadata); err != nil {
		createErr := fmt.Errorf("create metadata: %w", err)

		rollbackErr := s.blobs.Delete(context.WithoutCancel(ctx), id)
		if rollbackErr != nil {
			return files.Metadata{}, errors.Join(
				createErr,
				fmt.Errorf("rollback blob: %w", rollbackErr),
			)
		}

		return files.Metadata{}, createErr
	}

	return metadata, nil
}
