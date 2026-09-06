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

	log.Fatal(server.ListenAndServe())
}
