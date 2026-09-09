package local

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Emulisy/Go-Cloud-Storage/internal/blob"
)

func TestLocalStoreDeleteRemovesBlob(t *testing.T) {
	root := t.TempDir()
	store, err := NewBlobStore(root)
	if err != nil {
		t.Fatalf("NewBlobStore() error: %v", err)
	}
	if _, err := store.Put(
		context.Background(),
		"file-123",
		bytes.NewBufferString("content"),
	); err != nil {
		t.Fatalf("Put() error: %v", err)
	}

	if err := store.Delete(context.Background(), "file-123"); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}

	_, err = os.Stat(filepath.Join(root, "file-123"))
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Stat(deleted blob) error = %v, want os.ErrNotExist", err)
	}
}

func TestLocalStoreDeleteMissingBlob(t *testing.T) {
	store, err := NewBlobStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewBlobStore() error: %v", err)
	}

	err = store.Delete(context.Background(), "missing")
	if !errors.Is(err, blob.ErrNotFound) {
		t.Errorf("Delete() error = %v, want blob.ErrNotFound", err)
	}
}

func TestLocalStoreDeleteRejectsInvalidKeys(t *testing.T) {
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
			err := store.Delete(context.Background(), key)
			if !errors.Is(err, blob.ErrInvalidKey) {
				t.Errorf("Delete() error = %v, want blob.ErrInvalidKey", err)
			}
		})
	}
}

func TestLocalStoreDeleteCancelled(t *testing.T) {
	root := t.TempDir()
	store, err := NewBlobStore(root)
	if err != nil {
		t.Fatalf("NewBlobStore() error: %v", err)
	}
	if _, err := store.Put(
		context.Background(),
		"file-123",
		bytes.NewBufferString("content"),
	); err != nil {
		t.Fatalf("Put() error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = store.Delete(ctx, "file-123")
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Delete() error = %v, want context.Canceled", err)
	}
	if _, err := os.Stat(filepath.Join(root, "file-123")); err != nil {
		t.Errorf("Stat(blob) after cancelled Delete() error: %v", err)
	}
}
