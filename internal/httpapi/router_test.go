package httpapi

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealth(t *testing.T) {
	// Arrange: construct an HTTP request and response recorder.
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	recorder := httptest.NewRecorder()

	handler := NewHandler()

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

	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}

	if string(body) != "{\"status\":\"ok\"}\n" {
		t.Errorf("body: got %q, want %q", string(body), "{\"status\":\"ok\"}\n")
	}
}

func TestUnknownRoute(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/does-not-exist", nil)

	handler := NewHandler()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	response := recorder.Result()

	defer response.Body.Close()

	if response.StatusCode != http.StatusNotFound {
		t.Errorf("status code: got %d, want %d",
			response.StatusCode,
			http.StatusNotFound,
		)
	}
}
