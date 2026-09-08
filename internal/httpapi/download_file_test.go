package httpapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	downloadservice "github.com/Emulisy/Go-Cloud-Storage/internal/download"
	"github.com/Emulisy/Go-Cloud-Storage/internal/files"
)

func TestDownloadFile(t *testing.T) {
	content := &trackedDownloadContent{Reader: strings.NewReader("hello")}
	var gotID string
	downloader := downloaderStub{
		download: func(
			ctx context.Context,
			id string,
		) (downloadservice.File, error) {
			gotID = id
			return downloadservice.File{
				Metadata: files.Metadata{
					ID:   "file-123",
					Name: "notes.txt",
					Size: 5,
				},
				Content: content,
			}, nil
		},
	}
	request := httptest.NewRequest(
		http.MethodGet,
		"/files/file-123/content",
		nil,
	)
	recorder := httptest.NewRecorder()

	newDownloadTestHandler(downloader).ServeHTTP(recorder, request)

	response := recorder.Result()
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf(
			"status code: got %d, want %d",
			response.StatusCode,
			http.StatusOK,
		)
	}
	if gotID != "file-123" {
		t.Errorf("Download() ID = %q, want %q", gotID, "file-123")
	}
	if got := response.Header.Get("Content-Type"); got != "application/octet-stream" {
		t.Errorf(
			"Content-Type: got %q, want %q",
			got,
			"application/octet-stream",
		)
	}
	if got := response.Header.Get("Content-Length"); got != "5" {
		t.Errorf("Content-Length: got %q, want %q", got, "5")
	}
	if got := response.Header.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options: got %q, want %q", got, "nosniff")
	}
	assertAttachmentFilename(t, response.Header, "notes.txt")

	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("ReadAll(response) error: %v", err)
	}
	if string(body) != "hello" {
		t.Errorf("response body = %q, want %q", body, "hello")
	}
	if content.closeCalls != 1 {
		t.Errorf("content Close() calls = %d, want 1", content.closeCalls)
	}
}

func TestDownloadFileMapsServiceErrors(t *testing.T) {
	tests := []struct {
		name       string
		serviceErr error
		wantStatus int
	}{
		{
			name:       "invalid ID",
			serviceErr: fmt.Errorf("wrapped: %w", downloadservice.ErrInvalidID),
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "not found",
			serviceErr: fmt.Errorf("wrapped: %w", downloadservice.ErrNotFound),
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "content unavailable",
			serviceErr: fmt.Errorf("wrapped: %w", downloadservice.ErrContentUnavailable),
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "internal failure",
			serviceErr: errors.New("secret storage failure"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			downloader := downloaderStub{
				download: func(
					ctx context.Context,
					id string,
				) (downloadservice.File, error) {
					return downloadservice.File{}, tt.serviceErr
				},
			}
			request := httptest.NewRequest(
				http.MethodGet,
				"/files/file-123/content",
				nil,
			)
			recorder := httptest.NewRecorder()

			newDownloadTestHandler(downloader).ServeHTTP(recorder, request)

			if recorder.Code != tt.wantStatus {
				t.Errorf(
					"status code: got %d, want %d",
					recorder.Code,
					tt.wantStatus,
				)
			}
			if strings.Contains(recorder.Body.String(), "secret storage failure") {
				t.Error("response exposed an internal error")
			}
			if got := recorder.Header().Get("Content-Disposition"); got != "" {
				t.Errorf("Content-Disposition = %q on an error response", got)
			}
		})
	}
}

func TestDownloadFileSanitizesFilename(t *testing.T) {
	tests := []struct {
		name string
		file string
		want string
	}{
		{name: "ordinary", file: "notes.txt", want: "notes.txt"},
		{name: "slash path", file: "../../secret.txt", want: "secret.txt"},
		{name: "backslash path", file: `..\secret.txt`, want: "secret.txt"},
		{
			name: "quotes",
			file: `quarterly "report".txt`,
			want: `quarterly "report".txt`,
		},
		{name: "unicode", file: "résumé.pdf", want: "résumé.pdf"},
		{
			name: "header injection",
			file: "report\r\nX-Evil: injected.txt",
			want: "reportX-Evil: injected.txt",
		},
		{name: "empty after cleaning", file: " \t ", want: "download"},
		{name: "parent directory", file: "..", want: "download"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := &trackedDownloadContent{Reader: strings.NewReader("")}
			downloader := downloaderStub{
				download: func(
					ctx context.Context,
					id string,
				) (downloadservice.File, error) {
					return downloadservice.File{
						Metadata: files.Metadata{
							ID:   "file-123",
							Name: tt.file,
							Size: 0,
						},
						Content: content,
					}, nil
				},
			}
			request := httptest.NewRequest(
				http.MethodGet,
				"/files/file-123/content",
				nil,
			)
			recorder := httptest.NewRecorder()

			newDownloadTestHandler(downloader).ServeHTTP(recorder, request)

			assertAttachmentFilename(t, recorder.Header(), tt.want)
			disposition := recorder.Header().Get("Content-Disposition")
			if strings.ContainsAny(disposition, "\r\n") {
				t.Errorf("Content-Disposition contains a line break: %q", disposition)
			}
			if got := recorder.Header().Get("X-Evil"); got != "" {
				t.Errorf("injected X-Evil header = %q", got)
			}
			if content.closeCalls != 1 {
				t.Errorf("content Close() calls = %d, want 1", content.closeCalls)
			}
		})
	}
}

func TestDownloadFileReadFailureDoesNotAppendHTTPError(t *testing.T) {
	wantErr := errors.New("content read failed")
	content := &trackedDownloadContent{
		Reader: &readerThatFails{
			data: []byte("partial"),
			err:  wantErr,
		},
	}
	downloader := downloaderStub{
		download: func(
			ctx context.Context,
			id string,
		) (downloadservice.File, error) {
			return downloadservice.File{
				Metadata: files.Metadata{
					ID:   "file-123",
					Name: "notes.txt",
					Size: 20,
				},
				Content: content,
			}, nil
		},
	}
	request := httptest.NewRequest(
		http.MethodGet,
		"/files/file-123/content",
		nil,
	)
	recorder := httptest.NewRecorder()

	newDownloadTestHandler(downloader).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("status code: got %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Body.String(); got != "partial" {
		t.Errorf("response body = %q, want only %q", got, "partial")
	}
	if content.closeCalls != 1 {
		t.Errorf("content Close() calls = %d, want 1", content.closeCalls)
	}
}

func TestDownloadFileWriteFailureClosesContentOnce(t *testing.T) {
	content := &trackedDownloadContent{Reader: strings.NewReader("hello")}
	downloader := downloaderStub{
		download: func(
			ctx context.Context,
			id string,
		) (downloadservice.File, error) {
			return downloadservice.File{
				Metadata: files.Metadata{
					ID:   "file-123",
					Name: "notes.txt",
					Size: 5,
				},
				Content: content,
			}, nil
		},
	}
	writer := &failingResponseWriter{header: make(http.Header)}
	request := httptest.NewRequest(
		http.MethodGet,
		"/files/file-123/content",
		nil,
	)

	newDownloadTestHandler(downloader).ServeHTTP(writer, request)

	if writer.writeCalls != 1 {
		t.Errorf("response Write() calls = %d, want 1", writer.writeCalls)
	}
	if content.closeCalls != 1 {
		t.Errorf("content Close() calls = %d, want 1", content.closeCalls)
	}
}

type trackedDownloadContent struct {
	io.Reader
	closeCalls int
}

func (c *trackedDownloadContent) Close() error {
	c.closeCalls++
	return nil
}

type readerThatFails struct {
	data []byte
	err  error
	done bool
}

func (r *readerThatFails) Read(p []byte) (int, error) {
	if r.done {
		return 0, r.err
	}
	r.done = true
	return copy(p, r.data), r.err
}

type failingResponseWriter struct {
	header     http.Header
	statusCode int
	writeCalls int
}

func (w *failingResponseWriter) Header() http.Header {
	return w.header
}

func (w *failingResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
}

func (w *failingResponseWriter) Write(p []byte) (int, error) {
	if w.statusCode == 0 {
		w.statusCode = http.StatusOK
	}
	w.writeCalls++
	return 0, errors.New("response write failed")
}

func newDownloadTestHandler(downloader FileDownloader) http.Handler {
	return NewHandler(
		files.NewMemoryStore(nil),
		uploaderStub{},
		downloader,
	)
}

func assertAttachmentFilename(
	t *testing.T,
	header http.Header,
	want string,
) {
	t.Helper()

	disposition := header.Get("Content-Disposition")
	mediaType, parameters, err := mime.ParseMediaType(disposition)
	if err != nil {
		t.Fatalf("parse Content-Disposition %q: %v", disposition, err)
	}
	if mediaType != "attachment" {
		t.Errorf("Content-Disposition type = %q, want %q", mediaType, "attachment")
	}
	if got := parameters["filename"]; got != want {
		t.Errorf("download filename = %q, want %q", got, want)
	}
}
