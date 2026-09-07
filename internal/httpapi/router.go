package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/Emulisy/Go-Cloud-Storage/internal/files"
)

type API struct {
	files files.Reader
}

func NewHandler(fileReader files.Reader) http.Handler {
	api := &API{files: fileReader}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", health)
	mux.HandleFunc("GET /files/{id}", api.getFileMetadata)

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
