package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Emulisy/Go-Cloud-Storage/internal/blob"
	"github.com/Emulisy/Go-Cloud-Storage/internal/files"
	uploadservice "github.com/Emulisy/Go-Cloud-Storage/internal/upload"
)

func TestUploadFileIntegrationPersistsMetadataAndContent(t *testing.T) {
	root := t.TempDir()
	blobStore, err := blob.NewLocalStore(root)
	if err != nil {
		t.Fatalf("NewLocalStore() error: %v", err)
	}
	metadataStore := files.NewMemoryStore(nil)
	handler := NewHandler(
		metadataStore,
		uploadservice.NewService(blobStore, metadataStore),
		downloaderStub{},
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
}
