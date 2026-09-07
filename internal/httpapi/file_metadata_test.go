package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Emulisy/Go-Cloud-Storage/internal/files"
)

func TestGetFileMetadata(t *testing.T) {
	request := httptest.NewRequest(
		http.MethodGet,
		"/files/file-123?include_checksum=true",
		nil,
	)
	recorder := httptest.NewRecorder()

	newTestHandler().ServeHTTP(recorder, request)

	response := recorder.Result()
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf(
			"status code: got %d, want %d",
			response.StatusCode,
			http.StatusOK,
		)
	}

	if got := response.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type: got %q, want %q", got, "application/json")
	}

	var metadata fileMetadataResponse
	if err := json.NewDecoder(response.Body).Decode(&metadata); err != nil {
		t.Fatalf("decode response body: %v", err)
	}

	if metadata.ID != "file-123" {
		t.Errorf("ID: got %q, want %q", metadata.ID, "file-123")
	}

	if metadata.Name != "notes.txt" {
		t.Errorf("Name: got %q, want %q", metadata.Name, "notes.txt")
	}

	if metadata.Size != 128 {
		t.Errorf("Size: got %d, want %d", metadata.Size, 128)
	}

	if metadata.Checksum != "sha256:example" {
		t.Errorf(
			"Checksum: got %q, want %q",
			metadata.Checksum,
			"sha256:example",
		)
	}
}

func TestGetFileMetadataWithoutChecksum(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/files/file-123", nil)
	recorder := httptest.NewRecorder()

	newTestHandler().ServeHTTP(recorder, request)

	response := recorder.Result()
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status code: got %d, want %d", response.StatusCode, http.StatusOK)
	}

	var body map[string]json.RawMessage
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}

	if _, exists := body["checksum"]; exists {
		t.Error("checksum should be omitted")
	}
}

func TestGetFileMetadataRejectsInvalidQuery(t *testing.T) {
	request := httptest.NewRequest(
		http.MethodGet,
		"/files/file-123?include_checksum=maybe",
		nil,
	)
	recorder := httptest.NewRecorder()

	newTestHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("status code: got %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestGetFileMetadataNotFound(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/files/missing", nil)
	recorder := httptest.NewRecorder()

	newTestHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Errorf("status code: got %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

func TestGetFileMetadataStoreFailure(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/files/file-123", nil)
	recorder := httptest.NewRecorder()
	store := failingReader{err: errors.New("store unavailable")}

	NewHandler(store).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf(
			"status code: got %d, want %d",
			recorder.Code,
			http.StatusInternalServerError,
		)
	}
}

type failingReader struct {
	err error
}

func (f failingReader) Get(context.Context, string) (files.Metadata, error) {
	return files.Metadata{}, f.err
}
