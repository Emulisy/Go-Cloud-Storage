package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"goCloudStorage/db"
	"goCloudStorage/meta"
	"goCloudStorage/util"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
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
		username, err := authenticatedUsername(r)
		if err != nil {
			http.Error(w, "please sign in before uploading", http.StatusUnauthorized)
			return
		}

		userID, err := db.GetUserID(username)
		if err != nil {
			if errors.Is(err, db.ErrInvalidCredentials) {
				http.Error(w, "please sign in before uploading", http.StatusUnauthorized)
				return
			}

			log.Printf("failed to get user ID for upload: %v", err)
			http.Error(w, "unable to identify user", http.StatusInternalServerError)
			return
		}

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
		reused, err := tryFastUpload(userID, fileMeta)
		if err != nil {
			log.Printf("failed to reuse uploaded file: %v", err)
			http.Error(w, "failed to check stored file", http.StatusInternalServerError)
			return
		}
		if reused {
			http.Redirect(w, r, "/file/home", http.StatusSeeOther)
			return
		}

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

		if err := meta.UpdateFileMetaDB(fileMeta); err != nil {
			log.Printf("failed to save uploaded file metadata: %v", err)
			http.Error(w, "failed to save file metadata", http.StatusInternalServerError)
			return
		}

		// Record which authenticated user uploaded this file.
		if err := db.OnUserFileUploadFinish(
			userID,
			fileMeta.FileSha256,
			fileMeta.FileName,
			fileMeta.FileSize,
		); err != nil {
			log.Printf("failed to save user-file relationship: %v", err)
			if rollbackErr := db.DeleteUploadedFileMeta(fileMeta.FileSha256); rollbackErr != nil {
				// Preserve the stored content if its database row could not be rolled back.
				uploadSucceeded = true
				log.Printf("failed to roll back file metadata: %v", rollbackErr)
			}
			http.Error(w, "failed to associate file with user", http.StatusInternalServerError)
			return
		}

		uploadSucceeded = true
		http.Redirect(w, r, "/file/home", http.StatusSeeOther)
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

	username, err := authenticatedUsername(r)
	if err != nil {
		http.Error(w, "please sign in", http.StatusUnauthorized)
		return
	}

	userID, err := db.GetUserID(username)
	if err != nil {
		if errors.Is(err, db.ErrInvalidCredentials) {
			http.Error(w, "please sign in", http.StatusUnauthorized)
			return
		}

		log.Printf("failed to get user ID for file metadata: %v", err)
		http.Error(w, "unable to retrieve file metadata", http.StatusInternalServerError)
		return
	}

	files, err := db.QueryUserFileMetas(userID, page, pageSize)
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

// tryFastUpload links existing content to the user without storing it again.
func tryFastUpload(userID int64, fileMeta meta.FileMeta) (bool, error) {
	storedFile, err := db.GetFileMeta(fileMeta.FileSha256)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("look up stored file: %w", err)
	}

	if !storedFile.FileAddr.Valid || storedFile.FileAddr.String == "" {
		return false, fmt.Errorf("stored file has no location")
	}
	info, err := os.Stat(storedFile.FileAddr.String)
	if err != nil {
		return false, fmt.Errorf("stat stored file: %w", err)
	}
	if !info.Mode().IsRegular() || !storedFile.FileSize.Valid || info.Size() != storedFile.FileSize.Int64 {
		return false, fmt.Errorf("stored file does not match metadata")
	}

	if err := db.OnUserFileUploadFinish(userID, fileMeta.FileSha256, fileMeta.FileName, info.Size()); err != nil {
		return false, fmt.Errorf("associate stored file with user: %w", err)
	}

	return true, nil
}
