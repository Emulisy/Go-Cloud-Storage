package httpapi

import "net/http"

// NewHandler registers the current file and health routes on an HTTP handler.
// Dependencies must be non-nil, safe for concurrent requests, and valid for the
// handler's lifetime. The handler does not own or close those dependencies.
func NewHandler(
	fileReader MetadataReader,
	uploader FileUploader,
	downloader FileDownloader,
) http.Handler {
	handler := &api{
		metadata:   fileReader,
		uploader:   uploader,
		downloader: downloader,
	}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", health)
	mux.HandleFunc("GET /files/{id}", handler.getFileMetadata)
	mux.HandleFunc("POST /files", handler.uploadFile)
	mux.HandleFunc("GET /files/{id}/content", handler.downloadFile)

	return mux
}
