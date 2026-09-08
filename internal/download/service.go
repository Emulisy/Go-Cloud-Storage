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
	ErrInvalidID          = errors.New("invalid file ID")
	ErrNotFound           = errors.New("file not found")
	ErrContentUnavailable = errors.New("file content unavailable")
)

// File contains metadata and an open content stream.
// The caller owns Content and must close it.
type File struct {
	Metadata files.Metadata
	Content  io.ReadCloser
}

// Service coordinates metadata lookup and blob access for downloads.
type Service struct {
	blobs    blob.Reader
	metadata files.Reader
}

func NewService(blobs blob.Reader, metadata files.Reader) *Service {
	return &Service{
		blobs:    blobs,
		metadata: metadata,
	}
}

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
