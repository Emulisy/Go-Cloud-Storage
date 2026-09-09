package httpapi

import (
	"context"
	"errors"
	"io"
	"net/http"

	downloadservice "github.com/Emulisy/Go-Cloud-Storage/internal/download"
	"github.com/Emulisy/Go-Cloud-Storage/internal/files"
	"github.com/Emulisy/Go-Cloud-Storage/internal/storage/memory"
)

func newTestHandler() http.Handler {
	store := memory.NewMetadataStore([]files.Metadata{{
		ID:       "file-123",
		Name:     "notes.txt",
		Size:     128,
		Checksum: "sha256:example",
	}})

	return NewHandler(store, uploaderStub{}, downloaderStub{})
}

type uploaderStub struct {
	upload func(
		ctx context.Context,
		name string,
		content io.Reader,
	) (files.Metadata, error)
}

func (s uploaderStub) Upload(
	ctx context.Context,
	name string,
	content io.Reader,
) (files.Metadata, error) {
	if s.upload == nil {
		return files.Metadata{}, errors.New("unexpected upload call")
	}
	return s.upload(ctx, name, content)
}

type downloaderStub struct {
	download func(ctx context.Context, id string) (downloadservice.File, error)
}

func (s downloaderStub) Download(
	ctx context.Context,
	id string,
) (downloadservice.File, error) {
	if s.download == nil {
		return downloadservice.File{}, errors.New("unexpected download call")
	}
	return s.download(ctx, id)
}
