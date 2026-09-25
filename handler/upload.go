package handler

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"goCloudStorage/auth"
	"goCloudStorage/db"
	"goCloudStorage/storage"
	"goCloudStorage/util"
	"io"
	"log"
	"net/http"
	"os"
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
	defer r.MultipartForm.RemoveAll()
	defer file.Close()

	// 2. Construct metadata and calculate hash.
	storedFile := db.StoredFile{}
	originalFileName := strings.TrimSpace(header.Filename)
	if originalFileName == "" || len(originalFileName) > 255 {
		http.Error(w, "invalid filename", http.StatusBadRequest)
		return
	}

	sha256, err := util.CalculateSHA256(file)
	if err != nil {
		http.Error(w, "failed to hash uploaded file", http.StatusInternalServerError)
		return
	}
	storedFile.Hash = sha256

	// 3. Reset the file reader.
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		http.Error(w, "failed to read uploaded file", http.StatusInternalServerError)
		return
	}

	// 4. Upload under a unique key so cleanup cannot delete another upload.
	key := "objects/" + storedFile.Hash + "/" + rand.Text()
	storedFile.Addr = "r2://" + key
	storedFile.Size = header.Size
	if err := storage.R2().PutObject(r.Context(), key, file, storedFile.Size, header.Header.Get("Content-Type")); err != nil {
		log.Printf("failed to upload R2 object %q: %v", key, err)
		http.Error(w, "failed to store uploaded file", http.StatusInternalServerError)
		return
	}

	// 5. Save file metadata to MySQL.
	contentCreated, err := db.StoreUserFile(
		user.UserID,
		originalFileName,
		storedFile,
	)
	if err != nil {
		// A commit error can have an unknown outcome. Retain the object rather
		// than risk deleting content referenced by a committed database row.
		log.Printf("failed to store user file; retained %q for reconciliation: %v", storedFile.Addr, err)
		http.Error(w, "failed to save uploaded file", http.StatusInternalServerError)
		return
	}

	// The database may have reused content uploaded earlier or concurrently.
	if !contentCreated {
		if err := removeStoredFile(r.Context(), storedFile.Addr); err != nil {
			log.Printf("failed to remove redundant object %q: %v", storedFile.Addr, err)
		}
	}

	// 6. Return the upload result.
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
	if fileName == "" || len(fileName) > 255 {
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
		r.Context(),
		user.UserID,
		fileName,
		fileHash,
		fileSize,
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
func tryFastUpload(ctx context.Context, userID int64, originalFileName string, fileHash string, fileSize int64) (bool, error) {
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
	if storedFile.Size != fileSize {
		return false, fmt.Errorf("requested file size does not match stored metadata")
	}
	if key, isR2 := strings.CutPrefix(storedFile.Addr, "r2://"); isR2 {
		info, err := storage.R2().HeadObject(ctx, key)
		if err != nil {
			return false, fmt.Errorf("head stored R2 object: %w", err)
		}
		if info.Size != storedFile.Size {
			return false, fmt.Errorf("stored R2 object does not match metadata")
		}
	} else {
		// Multipart uploads and older files still use local storage.
		info, err := os.Stat(storedFile.Addr)
		if err != nil {
			return false, fmt.Errorf("stat stored file: %w", err)
		}
		if !info.Mode().IsRegular() || info.Size() != storedFile.Size {
			return false, fmt.Errorf("stored file does not match metadata")
		}
	}

	if err := db.LinkUserFile(userID, originalFileName, fileHash); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("associate stored file with user: %w", err)
	}

	return true, nil
}
