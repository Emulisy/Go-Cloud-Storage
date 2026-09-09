package httpapi

import (
	"errors"
	"net/http"

	"github.com/Emulisy/Go-Cloud-Storage/internal/files"
)

func (a *api) getFileMetadata(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	includeChecksum := r.URL.Query().Get("include_checksum")
	if includeChecksum != "" && includeChecksum != "true" && includeChecksum != "false" {
		http.Error(w, "invalid include_checksum value", http.StatusBadRequest)
		return
	}

	metadata, err := a.metadata.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, files.ErrNotFound) {
			http.Error(w, "file not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Omit the checksum from the response copy without changing stored metadata.
	response := metadataResponse(metadata)
	if includeChecksum != "true" {
		response.Checksum = ""
	}

	writeJSON(w, http.StatusOK, response)
}
