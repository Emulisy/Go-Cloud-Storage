package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetFileMetadata(t *testing.T) {
	request := httptest.NewRequest(
		http.MethodGet,
		"/files/file-123?include_checksum=true",
		nil,
	)
	recorder := httptest.NewRecorder()

	NewHandler().ServeHTTP(recorder, request)

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
	t.Skip("TODO: omit include_checksum and verify checksum is absent")
}

func TestGetFileMetadataRejectsInvalidQuery(t *testing.T) {
	t.Skip("TODO: use include_checksum=maybe and expect 400")
}

func TestGetFileMetadataNotFound(t *testing.T) {
	t.Skip("TODO: request an unknown ID and expect 404")
}
