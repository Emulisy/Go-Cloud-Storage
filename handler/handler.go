package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"goCloudStorage/meta"
	"goCloudStorage/util"
)

// UploadHandler displays the upload page and accepts file uploads.
func UploadHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		data, err := os.ReadFile("static/view/index.html")
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(data)
	case http.MethodPost:
		// Receive the file from the request.
		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "invalid file upload", http.StatusBadRequest)
			return
		}
		defer file.Close()

		fileMeta := meta.FileMeta{
			FileName: header.Filename,
			UploadAt: time.Now(),
		}

		sha256, err := util.CalculateSHA256(file)
		if err != nil {
			http.Error(w, "failed to hash uploaded file", http.StatusInternalServerError)
			return
		}
		fileMeta.FileSha256 = sha256

		if _, err := file.Seek(0, io.SeekStart); err != nil {
			http.Error(w, "failed to read uploaded file", http.StatusInternalServerError)
			return
		}

		newFile, err := os.CreateTemp("", "gocloudstorage-*")
		if err != nil {
			http.Error(w, "failed to store uploaded file", http.StatusInternalServerError)
			return
		}
		fileMeta.Location = newFile.Name()
		uploadSucceeded := false
		defer func() {
			if !uploadSucceeded {
				_ = newFile.Close()
				_ = os.Remove(fileMeta.Location)
			}
		}()

		fileMeta.FileSize, err = io.Copy(newFile, file)
		if err != nil {
			http.Error(w, "failed to store uploaded file", http.StatusInternalServerError)
			return
		}
		if err := newFile.Close(); err != nil {
			http.Error(w, "failed to store uploaded file", http.StatusInternalServerError)
			return
		}

		meta.UpdateFileMeta(fileMeta)
		uploadSucceeded = true
		http.Redirect(w, r, "/file/upload/suc?sha256="+url.QueryEscape(fileMeta.FileSha256), http.StatusFound)
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// UploadSucHandler reports a successful file upload.
func UploadSucHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	data, err := os.ReadFile("static/view/success.html")
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

// GetFileMetaHnadler returns metadata as JSON.
func GetFileMetaHnadler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	fileHash := r.URL.Query().Get("fileHash")
	if fileHash == "" {
		http.Error(w, "fileHash is required", http.StatusBadRequest)
		return
	}

	fileMeta, err := meta.GetFileMeta(fileHash)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	data, err := json.Marshal(fileMeta)
	if err != nil {
		http.Error(w, "Failed to encode metadata", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data)
}

// Download the file from cloud
func DownloadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	filehash := r.URL.Query().Get("sha256")

	if filehash == "" {
		http.Error(w, "Missing sha256 fielhash", http.StatusBadRequest)
		return
	}

	fMeta, err := meta.GetFileMeta(filehash)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	file, err := os.Open(fMeta.Location)
	if err != nil {
		http.Error(w, "Can't retreive file from location", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	w.Header().Set("Content-Type", "application/octet-stream")

	w.Header().Set(
		"Content-Disposition",
		"attachment; filename=\""+fMeta.FileName+"\"",
	)

	_, err = io.Copy(w, file)
	if err != nil {
		http.Error(w, "Failed to send file", http.StatusInternalServerError)
		return
	}
}

// update filename
func FileUpdateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid update request", http.StatusBadRequest)
		return
	}

	fileHash := r.Form.Get("sha256")
	if fileHash == "" {
		http.Error(w, "Missing SHA-256", http.StatusBadRequest)
		return
	}

	newFileName := strings.TrimSpace(r.Form.Get("name"))
	if newFileName == "" {
		http.Error(w, "Missing new file name", http.StatusBadRequest)
		return
	}

	currentFM, err := meta.GetFileMeta(fileHash)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	currentFM.FileName = newFileName
	meta.UpdateFileMeta(currentFM)

	data, err := json.Marshal(currentFM)
	if err != nil {
		http.Error(w, "Failed to convert metadata", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func FileDelHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		w.Header().Set("Allow", "DELETE")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	fileHash := r.URL.Query().Get("sha256")

	if fileHash == "" {
		http.Error(w, "Missing sha256", http.StatusBadRequest)
		return
	}

	fm, err := meta.GetFileMeta(fileHash)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	err = os.Remove(fm.Location)
	if err != nil {
		http.Error(w, "Failed to delete file", http.StatusInternalServerError)
		return
	}

	err = meta.DeleteFileMeta(fileHash)
	if err != nil {
		http.Error(w, "Failed to delete metadata", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
