package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	downloadservice "github.com/Emulisy/Go-Cloud-Storage/internal/download"
	"github.com/Emulisy/Go-Cloud-Storage/internal/files"
)

type API struct {
	files      files.Reader
	uploader   FileUploader
	downloader FileDownloader
}

// FileUploader is the application behavior required by the upload endpoint.
type FileUploader interface {
	Upload(ctx context.Context, name string, content io.Reader) (files.Metadata, error)
}

// FileDownloader is the application behavior required by the download endpoint.
type FileDownloader interface {
	Download(ctx context.Context, id string) (downloadservice.File, error)
}

func NewHandler(
	fileReader files.Reader,
	uploader FileUploader,
	downloader FileDownloader,
) http.Handler {
	api := &API{
		files:      fileReader,
		uploader:   uploader,
		downloader: downloader,
	}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", health)
	mux.HandleFunc("GET /files/{id}", api.getFileMetadata)
	mux.HandleFunc("POST /files", api.uploadFile)
	mux.HandleFunc("GET /files/{id}/content", api.downloadFile)

	return mux
}

func health(w http.ResponseWriter, r *http.Request) {
	payload, err := json.Marshal(map[string]string{"status": "ok"})
	if err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(payload)
}
