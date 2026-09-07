package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealth(t *testing.T) {
	// Arrange: construct an HTTP request and response recorder.
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	recorder := httptest.NewRecorder()

	handler := newTestHandler()

	// Act: pass the request directly to the handler.
	handler.ServeHTTP(recorder, request)

	response := recorder.Result()
	defer response.Body.Close()

	// Assert: verify the HTTP status.
	if response.StatusCode != http.StatusOK {
		t.Errorf("status code: got %d, want %d",
			response.StatusCode,
			http.StatusOK,
		)
	}

	if contentType := response.Header.Get("Content-Type"); contentType != "application/json" {
		t.Errorf("Content-Type: got %q, want %q", contentType, "application/json")
	}

	var body map[string]string
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}

	if body["status"] != "ok" {
		t.Errorf("status: got %q, want %q", body["status"], "ok")
	}
}
