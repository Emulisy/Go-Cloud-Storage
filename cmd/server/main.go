package main

import (
	"log"
	"net/http"

	"github.com/Emulisy/Go-Cloud-Storage/internal/httpapi"
)

func main() {
	handler := httpapi.NewHandler()

	server := &http.Server{
		Addr:    ":8080",
		Handler: handler,
	}

	log.Printf("server listening on %s", server.Addr)

	// TODO:
	// Start the server.
	// If it returns an unexpected error, log it and terminate.
	log.Fatal(server.ListenAndServe())
}
