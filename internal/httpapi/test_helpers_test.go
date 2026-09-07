package httpapi

import (
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

	return NewHandler(store)
}
