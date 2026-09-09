package local

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/Emulisy/Go-Cloud-Storage/internal/blob"
)

func TestLocalStoreOpen(t *testing.T) {
	store, err := NewBlobStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewBlobStore() error: %v", err)
	}
	want := []byte("hello cloud storage")
	if _, err := store.Put(
		context.Background(),
		"file-123",
		bytes.NewReader(want),
	); err != nil {
		t.Fatalf("Put() error: %v", err)
	}

	content, err := store.Open(context.Background(), "file-123")
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer func() {
		if err := content.Close(); err != nil {
			t.Errorf("Close() error: %v", err)
		}
	}()

	got, err := io.ReadAll(content)
	if err != nil {
		t.Fatalf("ReadAll() error: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("Open() content = %q, want %q", got, want)
	}
}

func TestLocalStoreOpenMissingBlob(t *testing.T) {
	store, err := NewBlobStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewBlobStore() error: %v", err)
	}

	content, err := store.Open(context.Background(), "missing")
	if content != nil {
		_ = content.Close()
		t.Error("Open() content is non-nil after an error")
	}
	if !errors.Is(err, blob.ErrNotFound) {
		t.Errorf("Open() error = %v, want blob.ErrNotFound", err)
	}
}

func TestLocalStoreOpenRejectsInvalidKeys(t *testing.T) {
	store, err := NewBlobStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewBlobStore() error: %v", err)
	}
	keys := []string{
		"",
		"../escape",
		"nested/file",
		`nested\file`,
		"file name",
		"file.txt",
	}

	for _, key := range keys {
		t.Run(key, func(t *testing.T) {
			content, err := store.Open(context.Background(), key)
			if content != nil {
				_ = content.Close()
				t.Error("Open() content is non-nil after an error")
			}
			if !errors.Is(err, blob.ErrInvalidKey) {
				t.Errorf("Open() error = %v, want blob.ErrInvalidKey", err)
			}
		})
	}
}

func TestLocalStoreOpenCancelled(t *testing.T) {
	store, err := NewBlobStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewBlobStore() error: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	content, err := store.Open(ctx, "file-123")
	if content != nil {
		_ = content.Close()
		t.Error("Open() content is non-nil after an error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Open() error = %v, want context.Canceled", err)
	}
}
