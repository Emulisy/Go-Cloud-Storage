package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"math"
	"mime"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"goCloudStorage/auth"
	"goCloudStorage/db"
	"goCloudStorage/storage"
)

// GetFileMetaHandler returns the signed-in user's file metadata as JSON.
func GetFileMetaHandler(w http.ResponseWriter, r *http.Request, user auth.User) {
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

	files, err := db.ListUserFiles(user.UserID, page, pageSize)
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

// DownloadHandler streams an owned R2 object or an older local file.
func DownloadHandler(w http.ResponseWriter, r *http.Request, user auth.User) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userFileID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || userFileID < 1 {
		http.Error(w, "Invalid user file ID", http.StatusBadRequest)
		return
	}

	download, err := db.GetUserFileDownload(user.UserID, userFileID)
	if err != nil {
		if errors.Is(err, db.ErrUserFileNotFound) {
			http.Error(w, "File not found", http.StatusNotFound)
			return
		}

		log.Printf("failed to get user file for download: %v", err)
		http.Error(w, "Unable to download file", http.StatusInternalServerError)
		return
	}

	var file io.ReadCloser
	var size int64 = -1
	if key, isR2 := strings.CutPrefix(download.FileAddr, "r2://"); isR2 {
		var object *storage.Object
		object, err = storage.R2().GetObject(r.Context(), key)
		if err == nil {
			file = object.Body
			size = object.Size
		}
	} else {
		file, err = os.Open(download.FileAddr)
	}
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotFound) || errors.Is(err, os.ErrNotExist) {
			http.Error(w, "File not found", http.StatusNotFound)
			return
		}
		log.Printf("failed to open download %q: %v", download.FileAddr, err)
		http.Error(w, "Unable to download file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Cache-Control", "no-store")
	if size >= 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	}

	w.Header().Set("Content-Disposition", mime.FormatMediaType(
		"attachment",
		map[string]string{"filename": download.FileName},
	))

	_, err = io.Copy(w, file)
	if err != nil {
		log.Printf("failed to stream download %q: %v", download.FileAddr, err)
		// Abort a partial response instead of appending error text to the file.
		panic(http.ErrAbortHandler)
	}
}

// FileUpdateHandler renames one user-specific file row.
func FileUpdateHandler(w http.ResponseWriter, r *http.Request, user auth.User) {
	if r.Method != http.MethodPatch {
		w.Header().Set("Allow", "PATCH")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid update request", http.StatusBadRequest)
		return
	}

	userFileID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || userFileID < 1 {
		http.Error(w, "Invalid user file ID", http.StatusBadRequest)
		return
	}

	newFileName := strings.TrimSpace(r.Form.Get("name"))
	if newFileName == "" || len(newFileName) > 255 {
		http.Error(w, "Invalid new file name", http.StatusBadRequest)
		return
	}

	if err := db.RenameUserFile(user.UserID, newFileName, userFileID); err != nil {
		if errors.Is(err, db.ErrUserFileNotFound) {
			http.Error(w, "File not found", http.StatusNotFound)
			return
		}

		log.Printf("failed to rename user file: %v", err)
		http.Error(w, "Failed to update file name", http.StatusInternalServerError)
		return
	}

	response := struct {
		ID       int64  `json:"id"`
		FileName string `json:"fileName"`
	}{
		ID:       userFileID,
		FileName: newFileName,
	}

	data, err := json.Marshal(response)
	if err != nil {
		http.Error(w, "Failed to convert metadata", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func FileDelHandler(w http.ResponseWriter, r *http.Request, user auth.User) {
	if r.Method != http.MethodDelete {
		w.Header().Set("Allow", "DELETE")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userFileID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || userFileID < 1 {
		http.Error(w, "Invalid user file ID", http.StatusBadRequest)
		return
	}

	cleanupPath, err := db.DeleteUserFile(user.UserID, userFileID)
	if err != nil {
		if errors.Is(err, db.ErrUserFileNotFound) {
			http.Error(w, "File not found", http.StatusNotFound)
			return
		}

		log.Printf("failed to delete user file: %v", err)
		http.Error(w, "Unable to delete file", http.StatusInternalServerError)
		return
	}

	if cleanupPath != "" {
		if err := removeStoredFile(r.Context(), cleanupPath); err != nil {
			// The database deletion is already committed, so this is an orphaned-file
			// cleanup failure rather than a failed user deletion.
			log.Printf("failed to remove unreferenced file %q: %v", cleanupPath, err)
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// removeStoredFile supports R2 addresses and existing local/multipart files.
func removeStoredFile(ctx context.Context, addr string) error {
	if key, isR2 := strings.CutPrefix(addr, "r2://"); isR2 {
		// Cleanup must still run if the client disconnects after the DB commits.
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		return storage.R2().DeleteObject(cleanupCtx, key)
	}
	if err := os.Remove(addr); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
