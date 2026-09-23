package handler

import (
	"encoding/json"
	"goCloudStorage/db"
	"goCloudStorage/meta"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
)

// GetFileMetaHnadler returns the signed-in user's file metadata as JSON.
func GetFileMetaHnadler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	page := 1
	if rawPage := r.URL.Query().Get("page"); rawPage != "" {
		parsedPage, err := strconv.Atoi(rawPage)
		if err != nil || parsedPage < 1 {
			http.Error(w, "page must be a positive integer", http.StatusBadRequest)
			return
		}
		page = parsedPage
	}

	pageSize := 20
	if rawPageSize := r.URL.Query().Get("pageSize"); rawPageSize != "" {
		parsedPageSize, err := strconv.Atoi(rawPageSize)
		if err != nil || parsedPageSize < 1 {
			http.Error(w, "pageSize must be a positive integer", http.StatusBadRequest)
			return
		}
		pageSize = parsedPageSize
	}
	if page-1 > math.MaxInt/pageSize {
		http.Error(w, "page and pageSize are too large", http.StatusBadRequest)
		return
	}

	username, ok := usernameFromContext(r)
	if !ok {
		http.Error(w, "please sign in", http.StatusUnauthorized)
		return
	}

	files, err := db.QueryUserFileMetas(username, page, pageSize)
	if err != nil {
		log.Printf("failed to query user files: %v", err)
		http.Error(w, "unable to retrieve file metadata", http.StatusInternalServerError)
		return
	}

	data, err := json.Marshal(files)
	if err != nil {
		http.Error(w, "unable to encode file metadata", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
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

	fMeta, err := meta.GetFileMetaDB(filehash)
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

	currentFM, err := meta.GetFileMetaDB(fileHash)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	currentFM.FileName = newFileName
	meta.UpdateFileMetaDB(currentFM)

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

	fm, err := meta.GetFileMetaDB(fileHash)
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