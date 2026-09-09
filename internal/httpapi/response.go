package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/Emulisy/Go-Cloud-Storage/internal/files"
)

// fileMetadataResponse keeps the HTTP representation separate from the domain.
type fileMetadataResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	Checksum string `json:"checksum,omitempty"`
}

func metadataResponse(metadata files.Metadata) fileMetadataResponse {
	return fileMetadataResponse{
		ID:       metadata.ID,
		Name:     metadata.Name,
		Size:     metadata.Size,
		Checksum: metadata.Checksum,
	}
}

// writeJSON sends an encoded value with the given status. Encoding failures can
// still produce an HTTP error; write failures occur after headers are committed.
func writeJSON(w http.ResponseWriter, status int, value any) {
	// Encode before sending headers so encoding errors can still return HTTP 500.
	payload, err := json.Marshal(value)
	if err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(payload)
}
