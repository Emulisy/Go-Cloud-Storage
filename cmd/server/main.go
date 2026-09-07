package main

import (
	"log"
	"net/http"

	"github.com/Emulisy/Go-Cloud-Storage/internal/files"
	"github.com/Emulisy/Go-Cloud-Storage/internal/httpapi"
)

func main() {
	fileReader := files.NewMemoryStore([]files.Metadata{{
		ID:       "file-123",
		Name:     "notes.txt",
		Size:     128,
		Checksum: "sha256:example",
	}})
	handler := httpapi.NewHandler(fileReader)

	server := &http.Server{
		Addr:    ":8080",
		Handler: handler,
	}

	log.Printf("server listening on %s", server.Addr)

	log.Fatal(server.ListenAndServe())
}
