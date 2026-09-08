package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Emulisy/Go-Cloud-Storage/internal/files"
	uploadservice "github.com/Emulisy/Go-Cloud-Storage/internal/upload"
)

func TestUploadFile(t *testing.T) {
	want := files.Metadata{
		ID:       "file-123",
		Name:     "notes.txt",
		Size:     5,
		Checksum: "sha256:example",
	}
	uploader := &recordingUploader{metadata: want}
	handler := NewHandler(files.NewMemoryStore(nil), uploader, downloaderStub{})
	request := newMultipartUploadRequest(t, "notes.txt", []byte("hello"))
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
	if got := response.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type: got %q, want %q", got, "application/json")
	}
	if got := response.Header.Get("Location"); got != "/files/file-123" {
		t.Errorf("Location: got %q, want %q", got, "/files/file-123")
	}

	var body fileMetadataResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	wantBody := fileMetadataResponse{
		ID:       want.ID,
		Name:     want.Name,
		Size:     want.Size,
		Checksum: want.Checksum,
	}
	if body != wantBody {
		t.Errorf("response body = %+v, want %+v", body, wantBody)
	}

	if uploader.calls != 1 {
		t.Fatalf("Upload() calls = %d, want 1", uploader.calls)
	}
	if uploader.name != "notes.txt" {
		t.Errorf("Upload() name = %q, want %q", uploader.name, "notes.txt")
	}
	if !bytes.Equal(uploader.content, []byte("hello")) {
		t.Errorf("Upload() content = %q, want %q", uploader.content, "hello")
	}
}

func TestUploadFileRejectsNonMultipartRequest(t *testing.T) {
	uploader := &recordingUploader{}
	handler := NewHandler(files.NewMemoryStore(nil), uploader, downloaderStub{})
	request := httptest.NewRequest(
		http.MethodPost,
		"/files",
		bytes.NewBufferString("not multipart"),
	)
	request.Header.Set("Content-Type", "text/plain")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf(
			"status code: got %d, want %d",
			recorder.Code,
			http.StatusBadRequest,
		)
	}
	if uploader.calls != 0 {
		t.Errorf("Upload() calls = %d, want 0", uploader.calls)
	}
}

func TestUploadFileRequiresFileField(t *testing.T) {
	uploader := &recordingUploader{}
	handler := NewHandler(files.NewMemoryStore(nil), uploader, downloaderStub{})
	request := newMultipartRequestWithoutFile(t)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf(
			"status code: got %d, want %d",
			recorder.Code,
			http.StatusBadRequest,
		)
	}
	if uploader.calls != 0 {
		t.Errorf("Upload() calls = %d, want 0", uploader.calls)
	}
}

func TestUploadFileRejectsOversizedRequest(t *testing.T) {
	uploader := &recordingUploader{}
	handler := NewHandler(files.NewMemoryStore(nil), uploader, downloaderStub{})
	content := bytes.Repeat([]byte("x"), int(maxUploadRequestBytes))
	request := newMultipartUploadRequest(t, "large.bin", content)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Errorf(
			"status code: got %d, want %d",
			recorder.Code,
			http.StatusRequestEntityTooLarge,
		)
	}
	if uploader.calls != 0 {
		t.Errorf("Upload() calls = %d, want 0", uploader.calls)
	}
}

func TestUploadFileMapsServiceErrors(t *testing.T) {
	tests := []struct {
		name       string
		serviceErr error
		wantStatus int
	}{
		{
			name:       "invalid name",
			serviceErr: uploadservice.ErrInvalidName,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid content",
			serviceErr: uploadservice.ErrInvalidContent,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "internal failure",
			serviceErr: errors.New("storage failed"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uploader := &recordingUploader{err: tt.serviceErr}
			handler := NewHandler(
				files.NewMemoryStore(nil),
				uploader,
				downloaderStub{},
			)
			request := newMultipartUploadRequest(
				t,
				"notes.txt",
				[]byte("hello"),
			)
			recorder := httptest.NewRecorder()

			handler.ServeHTTP(recorder, request)

			if recorder.Code != tt.wantStatus {
				t.Errorf(
					"status code: got %d, want %d",
					recorder.Code,
					tt.wantStatus,
				)
			}
		})
	}
}

type recordingUploader struct {
	metadata files.Metadata
	err      error
	calls    int
	name     string
	content  []byte
}

func (u *recordingUploader) Upload(
	ctx context.Context,
	name string,
	content io.Reader,
) (files.Metadata, error) {
	u.calls++
	u.name = name

	bytes, err := io.ReadAll(content)
	if err != nil {
		return files.Metadata{}, err
	}
	u.content = bytes

	if u.err != nil {
		return files.Metadata{}, u.err
	}
	return u.metadata, nil
}

func newMultipartUploadRequest(
	t *testing.T,
	filename string,
	content []byte,
) *http.Request {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("CreateFormFile() error: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write multipart content: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/files", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}

func newMultipartRequestWithoutFile(t *testing.T) *http.Request {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("description", "missing file"); err != nil {
		t.Fatalf("WriteField() error: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/files", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}
