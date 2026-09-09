package httpapi

import (
	"errors"
	"net/http"

	"github.com/Emulisy/Go-Cloud-Storage/internal/upload"
)

// maxUploadRequestBytes limits the complete multipart body, including overhead.
const maxUploadRequestBytes int64 = 1 << 20

func (a *api) uploadFile(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadRequestBytes)
	// Release multipart resources even when parsing or the upload fails.
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()

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
		if errors.Is(err, upload.ErrInvalidName) ||
			errors.Is(err, upload.ErrInvalidContent) {
			http.Error(w, "invalid upload", http.StatusBadRequest)
			return
		}

		http.Error(w, "failed to upload file", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Location", "/files/"+metadata.ID)
	writeJSON(w, http.StatusCreated, metadataResponse(metadata))
}
