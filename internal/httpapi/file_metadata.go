package httpapi

import (
	"encoding/json"
	"net/http"
)

// fileMetadataResponse is the JSON shape returned by the metadata endpoint.
// It is an HTTP response type, not yet the application's permanent domain model.
type fileMetadataResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	Checksum string `json:"checksum,omitempty"`
}

func getFileMetadata(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	// TODO 2: Call findSampleFile and return 404 when it is not found.
	file, found := findSampleFile(id)
	if !found {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}
	// TODO 3: Read the optional include_checksum query parameter.
	includeChecksum := r.URL.Query().Get("include_checksum")
	// TODO 4: If supplied, parse it as a boolean; return 400 when invalid.
	if includeChecksum != "" {
		if includeChecksum != "true" && includeChecksum != "false" {
			http.Error(w, "invalid include_checksum value", http.StatusBadRequest)
			return
		}
	}
	// TODO 5: Clear Checksum when include_checksum is false or omitted.
	if includeChecksum == "false" || includeChecksum == "" {
		file.Checksum = ""
	}
	// TODO 6: Return the metadata as JSON with status 200.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	encoder := json.NewEncoder(w)
	err := encoder.Encode(file)
	if err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// findSampleFile temporarily replaces a real metadata store.
// A later phase will inject a storage interface and remove this function.
func findSampleFile(id string) (fileMetadataResponse, bool) {
	if id != "file-123" {
		return fileMetadataResponse{}, false
	}

	return fileMetadataResponse{
		ID:       "file-123",
		Name:     "notes.txt",
		Size:     128,
		Checksum: "sha256:example",
	}, true
}
