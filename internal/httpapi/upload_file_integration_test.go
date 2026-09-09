package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	downloadservice "github.com/Emulisy/Go-Cloud-Storage/internal/download"
	"github.com/Emulisy/Go-Cloud-Storage/internal/files"
	"github.com/Emulisy/Go-Cloud-Storage/internal/storage/local"
	"github.com/Emulisy/Go-Cloud-Storage/internal/storage/memory"
	uploadservice "github.com/Emulisy/Go-Cloud-Storage/internal/upload"
)

func TestFileUploadMetadataDownloadIntegration(t *testing.T) {
	root := t.TempDir()
	blobStore, err := local.NewBlobStore(root)
	if err != nil {
		t.Fatalf("NewBlobStore() error: %v", err)
	}
	metadataStore := memory.NewMetadataStore(nil)
	handler := NewHandler(
		metadataStore,
		uploadservice.NewService(blobStore, metadataStore),
		downloadservice.NewService(blobStore, metadataStore),
	)
	content := []byte("hello cloud storage")
	request := newMultipartUploadRequest(t, "notes.txt", content)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	response := recorder.Result()
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf(
			"status code: got %d, want %d",
			response.StatusCode,
			http.StatusCreated,
		)
	}

	var created fileMetadataResponse
	if err := json.NewDecoder(response.Body).Decode(&created); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if created.ID == "" {
		t.Fatal("created file ID is empty")
	}
	if got := response.Header.Get("Location"); got != "/files/"+created.ID {
		t.Errorf("Location: got %q, want %q", got, "/files/"+created.ID)
	}

	sum := sha256.Sum256(content)
	want := files.Metadata{
		ID:       created.ID,
		Name:     "notes.txt",
		Size:     int64(len(content)),
		Checksum: fmt.Sprintf("sha256:%x", sum),
	}
	storedMetadata, err := metadataStore.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("Get() metadata error: %v", err)
	}
	if storedMetadata != want {
		t.Errorf("stored metadata = %+v, want %+v", storedMetadata, want)
	}

	storedContent, err := os.ReadFile(filepath.Join(root, created.ID))
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if !bytes.Equal(storedContent, content) {
		t.Errorf("stored content = %q, want %q", storedContent, content)
	}

	// The metadata endpoint and downloader must see the same record that the
	// upload service created through its narrower metadata interface.
	metadataRecorder := httptest.NewRecorder()
	handler.ServeHTTP(metadataRecorder, httptest.NewRequest(
		http.MethodGet, "/files/"+created.ID+"?include_checksum=true", nil,
	))
	if metadataRecorder.Code != http.StatusOK {
		t.Fatalf("metadata status = %d, want %d", metadataRecorder.Code, http.StatusOK)
	}
	var retrieved fileMetadataResponse
	if err := json.Unmarshal(metadataRecorder.Body.Bytes(), &retrieved); err != nil {
		t.Fatalf("decode retrieved metadata: %v", err)
	}
	if retrieved != created {
		t.Errorf("retrieved metadata = %+v, want %+v", retrieved, created)
	}

	downloadRecorder := httptest.NewRecorder()
	handler.ServeHTTP(downloadRecorder, httptest.NewRequest(
		http.MethodGet, "/files/"+created.ID+"/content", nil,
	))
	downloadResponse := downloadRecorder.Result()
	defer downloadResponse.Body.Close()
	if downloadResponse.StatusCode != http.StatusOK {
		t.Fatalf("download status = %d, want %d", downloadResponse.StatusCode, http.StatusOK)
	}
	downloaded, err := io.ReadAll(downloadResponse.Body)
	if err != nil {
		t.Fatalf("read downloaded content: %v", err)
	}
	if !bytes.Equal(downloaded, content) {
		t.Errorf("downloaded content = %q, want %q", downloaded, content)
	}
	assertAttachmentFilename(t, downloadResponse.Header, "notes.txt")
}
