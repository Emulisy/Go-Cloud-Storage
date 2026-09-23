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
		storedFile := db.StoredFile{}
		originalFileName := header.Filename

		sha256, err := util.CalculateSHA256(file)
		if err != nil {
			http.Error(w, "failed to hash uploaded file", http.StatusInternalServerError)
			return
		}
		storedFile.Hash = sha256

		//first try the fast upload
		reused, err := tryFastUpload(user.Username, originalFileName, storedFile.Hash)
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
		storedFile.Addr = newFile.Name()
		uploadSucceeded := false
		defer func() {
			if !uploadSucceeded {
				_ = newFile.Close()
				_ = os.Remove(storedFile.Addr)
			}
		}()

		storedFile.Size, err = io.Copy(newFile, file)
		if err != nil {
			http.Error(w, "failed to store uploaded file", http.StatusInternalServerError)
			return
		}
		if err := newFile.Close(); err != nil {
			http.Error(w, "failed to store uploaded file", http.StatusInternalServerError)
			return
		}

		contentCreated, err := db.StoreUserFile(
			user.Username,
			originalFileName,
			storedFile,
		)
		if err != nil {
			log.Printf("failed to store user file: %v", err)
			http.Error(w, "failed to save uploaded file", http.StatusInternalServerError)
			return
		}

		// A concurrent upload may have created the shared row first. In that case,
		// this request's temporary file is redundant and the deferred cleanup removes it.
		uploadSucceeded = contentCreated
		http.Redirect(w, r, "/file/home", http.StatusSeeOther)
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// tryFastUpload links existing content to the user without storing it again.
func tryFastUpload(username string, originalFileName string, fileHash string) (bool, error) {
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

	if err := db.LinkUserFile(username, originalFileName, fileHash); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("associate stored file with user: %w", err)
	}

	return true, nil
}
