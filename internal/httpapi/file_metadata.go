package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Emulisy/Go-Cloud-Storage/internal/files"
)

// fileMetadataResponse is the JSON shape returned by the metadata endpoint.
// It is an HTTP response type, not yet the application's permanent domain model.
type fileMetadataResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	Checksum string `json:"checksum,omitempty"`
}

func (a *API) getFileMetadata(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	includeChecksum := r.URL.Query().Get("include_checksum")
	if includeChecksum != "" && includeChecksum != "true" && includeChecksum != "false" {
		http.Error(w, "invalid include_checksum value", http.StatusBadRequest)
		return
	}

	metadata, err := a.files.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, files.ErrNotFound) {
			http.Error(w, "file not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	file := fileMetadataResponse{
		ID:       metadata.ID,
		Name:     metadata.Name,
		Size:     metadata.Size,
		Checksum: metadata.Checksum,
	}

	if includeChecksum != "true" {
		file.Checksum = ""
	}

	payload, err := json.Marshal(file)
	if err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	_, _ = w.Write(payload)
}
