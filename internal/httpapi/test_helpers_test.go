package httpapi

import (
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/Emulisy/Go-Cloud-Storage/internal/files"
)

func newTestHandler() http.Handler {
	store := files.NewMemoryStore([]files.Metadata{{
		ID:       "file-123",
		Name:     "notes.txt",
		Size:     128,
		Checksum: "sha256:example",
	}})

	return NewHandler(store, uploaderStub{})
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
