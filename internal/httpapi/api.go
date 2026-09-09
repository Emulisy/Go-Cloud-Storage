// Package httpapi adapts file operations to HTTP requests and responses.
// It owns transport validation and serialization, while injected interfaces
// supply application behavior and metadata access.
package httpapi

import (
	"context"
	"io"

	"github.com/Emulisy/Go-Cloud-Storage/internal/download"
	"github.com/Emulisy/Go-Cloud-Storage/internal/files"
)

// MetadataReader supplies the information required by the metadata endpoint.
// Missing records must be reported with files.ErrNotFound, directly or wrapped.
type MetadataReader interface {
	Get(ctx context.Context, id string) (files.Metadata, error)
}

// FileUploader is the application behavior required by the upload endpoint.
// The handler retains ownership of content; implementations must not close it.
type FileUploader interface {
	Upload(ctx context.Context, name string, content io.Reader) (files.Metadata, error)
}

// FileDownloader is the application behavior required by the download endpoint.
// A successful result must contain a non-nil Content stream, which the handler closes.
type FileDownloader interface {
	Download(ctx context.Context, id string) (download.File, error)
}

type api struct {
	metadata   MetadataReader
	uploader   FileUploader
	downloader FileDownloader
}
