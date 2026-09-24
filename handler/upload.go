package handler

import (
	"database/sql"
	"errors"
	"fmt"
	"goCloudStorage/auth"
	"goCloudStorage/db"
	"goCloudStorage/util"
	"io"
	"log"
	"net/http"
	"os"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"
)

// UploadHandler displays the upload page and accepts file uploads.
func UploadHandler(w http.ResponseWriter, r *http.Request, user auth.User) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 1. Receive the file from the request.
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "invalid file upload", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// 2. Construct metadata and calculate hash.
	storedFile := db.StoredFile{}
	originalFileName := header.Filename

	sha256, err := util.CalculateSHA256(file)
	if err != nil {
		http.Error(w, "failed to hash uploaded file", http.StatusInternalServerError)
		return
	}
	storedFile.Hash = sha256

	// 4. Reset the file reader.
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		http.Error(w, "failed to read uploaded file", http.StatusInternalServerError)
		return
	}

	// 5. Create a temporary file.
	newFile, err := os.CreateTemp("", "gocloudstorage-*")
	if err != nil {
		http.Error(w, "failed to store uploaded file", http.StatusInternalServerError)
		return
	}

	storedFile.Addr = newFile.Name()
	uploadSucceeded := false

	defer func() {
		if !uploadSucceeded {
			_ = newFile.Close()
			_ = os.Remove(storedFile.Addr)
		}
	}()

	// 6. Copy the uploaded file to local storage.
	storedFile.Size, err = io.Copy(newFile, file)
	if err != nil {
		http.Error(w, "failed to store uploaded file", http.StatusInternalServerError)
		return
	}

	if err := newFile.Close(); err != nil {
		http.Error(w, "failed to store uploaded file", http.StatusInternalServerError)
		return
	}

	// 7. Save file metadata to MySQL.
	contentCreated, err := db.StoreUserFile(
		user.UserID,
		originalFileName,
		storedFile,
	)
	if err != nil {
		log.Printf("failed to store user file: %v", err)
		http.Error(w, "failed to save uploaded file", http.StatusInternalServerError)
		return
	}

	// A concurrent upload may have created the shared row first.
	// In that case, the temporary file is redundant and will be removed.
	uploadSucceeded = contentCreated

	// 8. Return the upload result.
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write([]byte("SUCCESS"))
}

// TryFastUploadHandler checks whether existing content can be reused.
func TryFastUploadHandler(w http.ResponseWriter, r *http.Request, user auth.User) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 1. Parse the request.
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	fileHash := strings.ToLower(strings.TrimSpace(r.PostForm.Get("filehash")))
	fileName := strings.TrimSpace(r.PostForm.Get("filename"))
	fileSize, err := strconv.ParseInt(r.PostForm.Get("filesize"), 10, 64)

	// 2. Validate file metadata.
	if fileName == "" {
		http.Error(w, "invalid filename", http.StatusBadRequest)
		return
	}

	if len(fileHash) != 64 {
		http.Error(w, "invalid file hash", http.StatusBadRequest)
		return
	}

	if _, err := hex.DecodeString(fileHash); err != nil {
		http.Error(w, "invalid file hash", http.StatusBadRequest)
		return
	}

	if err != nil || fileSize <= 0 {
		http.Error(w, "invalid file size", http.StatusBadRequest)
		return
	}

	// 3. Try fast upload.
	reused, err := tryFastUpload(
		user.UserID,
		fileName,
		fileHash,
	)
	if err != nil {
		log.Printf("failed to reuse uploaded file: %v", err)
		http.Error(w, "failed to check stored file", http.StatusInternalServerError)
		return
	}

	// 4. Return the upload result.
	w.Header().Set("Content-Type", "application/json")

	if reused {
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"reused": true,
			"status": "completed",
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"reused": false,
		"status": "upload_required",
	})
}

// tryFastUpload links existing content to the user without storing it again.
func tryFastUpload(userID int64, originalFileName string, fileHash string) (bool, error) {
	storedFile, err := db.GetStoredFile(fileHash)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("look up stored file: %w", err)
	}

	if storedFile.Addr == "" {
		return false, fmt.Errorf("stored file has no location")
	}
	info, err := os.Stat(storedFile.Addr)
	if err != nil {
		return false, fmt.Errorf("stat stored file: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() != storedFile.Size {
		return false, fmt.Errorf("stored file does not match metadata")
	}

	if err := db.LinkUserFile(userID, originalFileName, fileHash); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("associate stored file with user: %w", err)
	}

	return true, nil
}
