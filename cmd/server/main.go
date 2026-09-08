package main

import (
	"log"
	"net/http"

	"github.com/Emulisy/Go-Cloud-Storage/internal/blob"
	"github.com/Emulisy/Go-Cloud-Storage/internal/files"
	"github.com/Emulisy/Go-Cloud-Storage/internal/httpapi"
	"github.com/Emulisy/Go-Cloud-Storage/internal/upload"
)

func main() {
	metadataStore := files.NewMemoryStore([]files.Metadata{{
		ID:       "file-123",
		Name:     "notes.txt",
		Size:     128,
		Checksum: "sha256:example",
	}})

	blobStore, err := blob.NewLocalStore("data/blobs")
	if err != nil {
		log.Fatalf("create blob store: %v", err)
	}

	uploadService := upload.NewService(blobStore, metadataStore)
	handler := httpapi.NewHandler(metadataStore, uploadService)

	server := &http.Server{
		Addr:    ":8080",
		Handler: handler,
	}

	log.Printf("server listening on %s", server.Addr)

	log.Fatal(server.ListenAndServe())
}
