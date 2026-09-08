package download

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/Emulisy/Go-Cloud-Storage/internal/blob"
	"github.com/Emulisy/Go-Cloud-Storage/internal/files"
)

func TestServiceDownload(t *testing.T) {
	ctx := context.Background()
	order := []string{}
	wantMetadata := files.Metadata{
		ID:       "file-123",
		Name:     "notes.txt",
		Size:     5,
		Checksum: "sha256:example",
	}
	stream := &trackedReadCloser{Reader: strings.NewReader("hello")}
	metadata := &fakeMetadataReader{
		metadata: wantMetadata,
		order:    &order,
	}
	blobs := &fakeBlobReader{
		content: stream,
		order:   &order,
	}
	service := NewService(blobs, metadata)

	got, err := service.Download(ctx, "file-123")
	if err != nil {
		t.Fatalf("Download() error: %v", err)
	}
	if got.Metadata != wantMetadata {
		t.Errorf("Download() metadata = %+v, want %+v", got.Metadata, wantMetadata)
	}
	if got.Content != stream {
		t.Error("Download() returned a different content stream")
	}
	if stream.closed {
		t.Error("Download() closed the stream before returning it")
	}
	if metadata.calls != 1 || metadata.id != "file-123" {
		t.Errorf(
			"metadata Get() = %d call(s) with %q, want 1 with %q",
			metadata.calls,
			metadata.id,
			"file-123",
		)
	}
	if blobs.calls != 1 || blobs.key != "file-123" {
		t.Errorf(
			"blob Open() = %d call(s) with %q, want 1 with %q",
			blobs.calls,
			blobs.key,
			"file-123",
		)
	}
	if len(order) != 2 || order[0] != "metadata" || order[1] != "blob" {
		t.Errorf("dependency call order = %v, want [metadata blob]", order)
	}

	content, err := io.ReadAll(got.Content)
	if err != nil {
		t.Fatalf("ReadAll() error: %v", err)
	}
	if string(content) != "hello" {
		t.Errorf("content = %q, want %q", content, "hello")
	}
	if err := got.Content.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}
	if !stream.closed {
		t.Error("Close() did not close the returned stream")
	}
}

func TestServiceDownloadRejectsBlankID(t *testing.T) {
	metadata := &fakeMetadataReader{}
	blobs := &fakeBlobReader{}
	service := NewService(blobs, metadata)

	got, err := service.Download(context.Background(), "   ")
	if !errors.Is(err, ErrInvalidID) {
		t.Errorf("Download() error = %v, want ErrInvalidID", err)
	}
	assertEmptyFile(t, got)
	assertNoDownloadCalls(t, metadata, blobs)
}

func TestServiceDownloadCancelled(t *testing.T) {
	metadata := &fakeMetadataReader{}
	blobs := &fakeBlobReader{}
	service := NewService(blobs, metadata)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	got, err := service.Download(ctx, "file-123")
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Download() error = %v, want context.Canceled", err)
	}
	assertEmptyFile(t, got)
	assertNoDownloadCalls(t, metadata, blobs)
}

func TestServiceDownloadMetadataNotFound(t *testing.T) {
	metadata := &fakeMetadataReader{
		err: errors.Join(errors.New("lookup failed"), files.ErrNotFound),
	}
	blobs := &fakeBlobReader{}
	service := NewService(blobs, metadata)

	got, err := service.Download(context.Background(), "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Download() error = %v, want ErrNotFound", err)
	}
	assertEmptyFile(t, got)
	if metadata.calls != 1 {
		t.Errorf("metadata Get() calls = %d, want 1", metadata.calls)
	}
	if blobs.calls != 0 {
		t.Errorf("blob Open() calls = %d, want 0", blobs.calls)
	}
}

func TestServiceDownloadMetadataFailure(t *testing.T) {
	wantErr := errors.New("metadata unavailable")
	metadata := &fakeMetadataReader{err: wantErr}
	blobs := &fakeBlobReader{}
	service := NewService(blobs, metadata)

	got, err := service.Download(context.Background(), "file-123")
	if !errors.Is(err, wantErr) {
		t.Errorf("Download() error = %v, want wrapped metadata error", err)
	}
	assertEmptyFile(t, got)
	if blobs.calls != 0 {
		t.Errorf("blob Open() calls = %d, want 0", blobs.calls)
	}
}

func TestServiceDownloadMissingContent(t *testing.T) {
	metadata := &fakeMetadataReader{
		metadata: files.Metadata{ID: "file-123", Name: "notes.txt"},
	}
	blobs := &fakeBlobReader{
		err: errors.Join(errors.New("open failed"), blob.ErrNotFound),
	}
	service := NewService(blobs, metadata)

	got, err := service.Download(context.Background(), "file-123")
	if !errors.Is(err, ErrContentUnavailable) {
		t.Errorf("Download() error = %v, want ErrContentUnavailable", err)
	}
	if errors.Is(err, ErrNotFound) {
		t.Errorf("Download() error = %v, should not be ErrNotFound", err)
	}
	assertEmptyFile(t, got)
}

func TestServiceDownloadBlobFailure(t *testing.T) {
	wantErr := errors.New("blob unavailable")
	metadata := &fakeMetadataReader{
		metadata: files.Metadata{ID: "file-123", Name: "notes.txt"},
	}
	blobs := &fakeBlobReader{err: wantErr}
	service := NewService(blobs, metadata)

	got, err := service.Download(context.Background(), "file-123")
	if !errors.Is(err, wantErr) {
		t.Errorf("Download() error = %v, want wrapped blob error", err)
	}
	assertEmptyFile(t, got)
}

type fakeMetadataReader struct {
	metadata files.Metadata
	err      error
	calls    int
	id       string
	order    *[]string
}

func (r *fakeMetadataReader) Get(
	ctx context.Context,
	id string,
) (files.Metadata, error) {
	r.calls++
	r.id = id
	if r.order != nil {
		*r.order = append(*r.order, "metadata")
	}
	if r.err != nil {
		return files.Metadata{}, r.err
	}
	return r.metadata, nil
}

type fakeBlobReader struct {
	content io.ReadCloser
	err     error
	calls   int
	key     string
	order   *[]string
}

func (r *fakeBlobReader) Open(
	ctx context.Context,
	key string,
) (io.ReadCloser, error) {
	r.calls++
	r.key = key
	if r.order != nil {
		*r.order = append(*r.order, "blob")
	}
	if r.err != nil {
		return nil, r.err
	}
	return r.content, nil
}

type trackedReadCloser struct {
	io.Reader
	closed bool
}

func (r *trackedReadCloser) Close() error {
	r.closed = true
	return nil
}

func assertEmptyFile(t *testing.T, file File) {
	t.Helper()

	if file.Metadata != (files.Metadata{}) {
		t.Errorf("metadata = %+v, want zero value", file.Metadata)
	}
	if file.Content != nil {
		_ = file.Content.Close()
		t.Error("content is non-nil after an error")
	}
}

func assertNoDownloadCalls(
	t *testing.T,
	metadata *fakeMetadataReader,
	blobs *fakeBlobReader,
) {
	t.Helper()

	if metadata.calls != 0 {
		t.Errorf("metadata Get() calls = %d, want 0", metadata.calls)
	}
	if blobs.calls != 0 {
		t.Errorf("blob Open() calls = %d, want 0", blobs.calls)
	}
}
