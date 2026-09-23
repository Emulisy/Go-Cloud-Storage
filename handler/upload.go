package handler

import (
	"database/sql"
	"errors"
	"fmt"
	"goCloudStorage/auth"
	"goCloudStorage/db"
	"goCloudStorage/meta"
	"goCloudStorage/util"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

// UploadHandler displays the upload page and accepts file uploads.
func UploadHandler(w http.ResponseWriter, r *http.Request, user auth.User) {
	switch r.Method {
	case http.MethodGet: //get the upload page
		data, err := os.ReadFile("static/view/index.html")
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(data)
	case http.MethodPost: //upload a file
		// Receive the file from the request.
		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "invalid file upload", http.StatusBadRequest)
			return
		}
		defer file.Close()

		//construct metadata and caulculate hash
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

		//first try the fast upload
		reused, err := tryFastUpload(user.Username, fileMeta)
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
			user.Username,
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

// tryFastUpload links existing content to the user without storing it again.
func tryFastUpload(username string, fileMeta meta.FileMeta) (bool, error) {
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

	if err := db.OnUserFileUploadFinish(username, fileMeta.FileSha256, fileMeta.FileName, info.Size()); err != nil {
		return false, fmt.Errorf("associate stored file with user: %w", err)
	}

	return true, nil
}
