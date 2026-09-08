package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	uploadservice "github.com/Emulisy/Go-Cloud-Storage/internal/upload"
)

const maxUploadRequestBytes int64 = 1 << 20

func (a *API) uploadFile(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadRequestBytes)

	file, header, err := r.FormFile("file")
	if err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}

		http.Error(w, "failed to read form file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	metadata, err := a.uploader.Upload(r.Context(), header.Filename, file)
	if err != nil {
		if errors.Is(err, uploadservice.ErrInvalidName) ||
			errors.Is(err, uploadservice.ErrInvalidContent) {
			http.Error(w, "invalid upload", http.StatusBadRequest)
			return
		}

		http.Error(w, "failed to upload file", http.StatusInternalServerError)
		return
	}

	response := fileMetadataResponse{
		ID:       metadata.ID,
		Name:     metadata.Name,
		Size:     metadata.Size,
		Checksum: metadata.Checksum,
	}

	payload, err := json.Marshal(response)
	if err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Location", "/files/"+metadata.ID)
	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write(payload)
}
