package httpapi

import (
	"errors"
	"io"
	"log"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
	"unicode"

	"github.com/Emulisy/Go-Cloud-Storage/internal/download"
)

func (a *api) downloadFile(w http.ResponseWriter, r *http.Request) {
	file, err := a.downloader.Download(r.Context(), r.PathValue("id"))
	if err != nil {
		switch {
		case errors.Is(err, download.ErrInvalidID):
			http.Error(w, "invalid file ID", http.StatusBadRequest)
		case errors.Is(err, download.ErrNotFound):
			http.Error(w, "file not found", http.StatusNotFound)
		default:
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}
	defer file.Content.Close()

	filename := safeDownloadName(file.Metadata.Name)
	disposition := mime.FormatMediaType(
		"attachment",
		map[string]string{"filename": filename},
	)
	if disposition == "" {
		disposition = "attachment"
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(file.Metadata.Size, 10))
	w.Header().Set("Content-Disposition", disposition)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)

	if _, err := io.Copy(w, file.Content); err != nil {
		// Headers have been sent; an HTTP error would corrupt the download body.
		log.Printf("stream file %q: %v", file.Metadata.ID, err)
	}
}

// safeDownloadName derives an attachment display name from untrusted metadata.
// It is header presentation logic, not validation of a filesystem storage key.
func safeDownloadName(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	name = path.Base(name)
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, name)
	name = strings.TrimSpace(name)

	if name == "" || name == "." || name == ".." {
		return "download"
	}
	return name
}
