package upload

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/Emulisy/Go-Cloud-Storage/internal/blob"
	"github.com/Emulisy/Go-Cloud-Storage/internal/files"
)

func TestServiceUpload(t *testing.T) {
	blobs := &fakeBlobStore{
		putResult: blob.PutResult{
			Size:     5,
			Checksum: "sha256:example",
		},
	}
	metadata := &fakeMetadataWriter{}
	service := newService(blobs, metadata, fixedID("file-123"))

	got, err := service.Upload(
		context.Background(),
		"notes.txt",
		strings.NewReader("hello"),
	)
	if err != nil {
		t.Fatalf("Upload() error: %v", err)
	}

	want := files.Metadata{
		ID:       "file-123",
		Name:     "notes.txt",
		Size:     5,
		Checksum: "sha256:example",
	}
	if got != want {
		t.Errorf("Upload() metadata = %+v, want %+v", got, want)
	}
	if blobs.putCalls != 1 {
		t.Errorf("blob Put() calls = %d, want 1", blobs.putCalls)
	}
	if blobs.putKey != "file-123" {
		t.Errorf("blob Put() key = %q, want %q", blobs.putKey, "file-123")
	}
	if !bytes.Equal(blobs.putContent, []byte("hello")) {
		t.Errorf("blob Put() content = %q, want %q", blobs.putContent, "hello")
	}
	if metadata.createCalls != 1 {
		t.Errorf("metadata Create() calls = %d, want 1", metadata.createCalls)
	}
	if metadata.created != want {
		t.Errorf("created metadata = %+v, want %+v", metadata.created, want)
	}
	if blobs.deleteCalls != 0 {
		t.Errorf("blob Delete() calls = %d, want 0", blobs.deleteCalls)
	}
}

func TestServiceUploadRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name     string
		fileName string
		content  io.Reader
		wantErr  error
	}{
		{
			name:     "blank name",
			fileName: "   ",
			content:  strings.NewReader("hello"),
			wantErr:  ErrInvalidName,
		},
		{
			name:     "nil content",
			fileName: "notes.txt",
			content:  nil,
			wantErr:  ErrInvalidContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blobs := &fakeBlobStore{}
			metadata := &fakeMetadataWriter{}
			service := newService(blobs, metadata, fixedID("file-123"))

			_, err := service.Upload(
				context.Background(),
				tt.fileName,
				tt.content,
			)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Upload() error = %v, want %v", err, tt.wantErr)
			}
			assertNoStorageCalls(t, blobs, metadata)
		})
	}
}

func TestServiceUploadCancelled(t *testing.T) {
	blobs := &fakeBlobStore{}
	metadata := &fakeMetadataWriter{}
	service := newService(blobs, metadata, fixedID("file-123"))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := service.Upload(ctx, "notes.txt", strings.NewReader("hello"))
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Upload() error = %v, want context.Canceled", err)
	}
	assertNoStorageCalls(t, blobs, metadata)
}

func TestServiceUploadIDGenerationFailure(t *testing.T) {
	wantErr := errors.New("random source failed")
	blobs := &fakeBlobStore{}
	metadata := &fakeMetadataWriter{}
	service := newService(
		blobs,
		metadata,
		func() (string, error) { return "", wantErr },
	)

	_, err := service.Upload(
		context.Background(),
		"notes.txt",
		strings.NewReader("hello"),
	)
	if !errors.Is(err, wantErr) {
		t.Errorf("Upload() error = %v, want wrapped generator error", err)
	}
	assertNoStorageCalls(t, blobs, metadata)
}

func TestServiceUploadBlobFailure(t *testing.T) {
	wantErr := errors.New("blob write failed")
	blobs := &fakeBlobStore{putErr: wantErr}
	metadata := &fakeMetadataWriter{}
	service := newService(blobs, metadata, fixedID("file-123"))

	_, err := service.Upload(
		context.Background(),
		"notes.txt",
		strings.NewReader("hello"),
	)
	if !errors.Is(err, wantErr) {
		t.Errorf("Upload() error = %v, want wrapped blob error", err)
	}
	if metadata.createCalls != 0 {
		t.Errorf("metadata Create() calls = %d, want 0", metadata.createCalls)
	}
	if blobs.deleteCalls != 0 {
		t.Errorf("blob Delete() calls = %d, want 0", blobs.deleteCalls)
	}
}

func TestServiceUploadMetadataFailureRollsBackBlob(t *testing.T) {
	wantErr := errors.New("metadata write failed")
	blobs := &fakeBlobStore{
		putResult: blob.PutResult{Size: 5, Checksum: "sha256:example"},
	}
	metadata := &fakeMetadataWriter{createErr: wantErr}
	service := newService(blobs, metadata, fixedID("file-123"))

	_, err := service.Upload(
		context.Background(),
		"notes.txt",
		strings.NewReader("hello"),
	)
	if !errors.Is(err, wantErr) {
		t.Errorf("Upload() error = %v, want wrapped metadata error", err)
	}
	if blobs.deleteCalls != 1 {
		t.Fatalf("blob Delete() calls = %d, want 1", blobs.deleteCalls)
	}
	if blobs.deleteKey != "file-123" {
		t.Errorf("blob Delete() key = %q, want %q", blobs.deleteKey, "file-123")
	}
}

func TestServiceUploadRollbackSurvivesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	blobs := &fakeBlobStore{
		putResult: blob.PutResult{Size: 5, Checksum: "sha256:example"},
	}
	metadata := &fakeMetadataWriter{
		createErr: context.Canceled,
		onCreate:  cancel,
	}
	service := newService(blobs, metadata, fixedID("file-123"))

	_, err := service.Upload(ctx, "notes.txt", strings.NewReader("hello"))
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Upload() error = %v, want context.Canceled", err)
	}
	if blobs.deleteCalls != 1 {
		t.Fatalf("blob Delete() calls = %d, want 1", blobs.deleteCalls)
	}
	if blobs.deleteContextErr != nil {
		t.Errorf("rollback context error = %v, want nil", blobs.deleteContextErr)
	}
}

func TestServiceUploadPreservesMetadataAndRollbackErrors(t *testing.T) {
	metadataErr := errors.New("metadata write failed")
	rollbackErr := errors.New("blob rollback failed")
	blobs := &fakeBlobStore{
		putResult: blob.PutResult{Size: 5, Checksum: "sha256:example"},
		deleteErr: rollbackErr,
	}
	metadata := &fakeMetadataWriter{createErr: metadataErr}
	service := newService(blobs, metadata, fixedID("file-123"))

	_, err := service.Upload(
		context.Background(),
		"notes.txt",
		strings.NewReader("hello"),
	)
	if !errors.Is(err, metadataErr) {
		t.Errorf("Upload() error = %v, want metadata error preserved", err)
	}
	if !errors.Is(err, rollbackErr) {
		t.Errorf("Upload() error = %v, want rollback error preserved", err)
	}
}

type fakeBlobStore struct {
	putResult        blob.PutResult
	putErr           error
	putCalls         int
	putKey           string
	putContent       []byte
	deleteErr        error
	deleteCalls      int
	deleteKey        string
	deleteContextErr error
}

func (s *fakeBlobStore) Put(
	ctx context.Context,
	key string,
	source io.Reader,
) (blob.PutResult, error) {
	s.putCalls++
	s.putKey = key

	content, err := io.ReadAll(source)
	if err != nil {
		return blob.PutResult{}, err
	}
	s.putContent = content

	if s.putErr != nil {
		return blob.PutResult{}, s.putErr
	}
	return s.putResult, nil
}

func (s *fakeBlobStore) Delete(ctx context.Context, key string) error {
	s.deleteCalls++
	s.deleteKey = key
	s.deleteContextErr = ctx.Err()

	if s.deleteContextErr != nil {
		return s.deleteContextErr
	}
	return s.deleteErr
}

type fakeMetadataWriter struct {
	createErr   error
	createCalls int
	created     files.Metadata
	onCreate    func()
}

func (w *fakeMetadataWriter) Create(
	ctx context.Context,
	metadata files.Metadata,
) error {
	w.createCalls++
	w.created = metadata
	if w.onCreate != nil {
		w.onCreate()
	}
	return w.createErr
}

func fixedID(id string) IDGenerator {
	return func() (string, error) {
		return id, nil
	}
}

func assertNoStorageCalls(
	t *testing.T,
	blobs *fakeBlobStore,
	metadata *fakeMetadataWriter,
) {
	t.Helper()

	if blobs.putCalls != 0 {
		t.Errorf("blob Put() calls = %d, want 0", blobs.putCalls)
	}
	if blobs.deleteCalls != 0 {
		t.Errorf("blob Delete() calls = %d, want 0", blobs.deleteCalls)
	}
	if metadata.createCalls != 0 {
		t.Errorf("metadata Create() calls = %d, want 0", metadata.createCalls)
	}
}
