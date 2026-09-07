package blob

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestNewLocalStoreCreatesRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "objects")

	_, err := NewLocalStore(root)
	if err != nil {
		t.Fatalf("NewLocalStore() error: %v", err)
	}

	info, err := os.Stat(root)
	if err != nil {
		t.Fatalf("Stat(root) error: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("root is not a directory")
	}
}

func TestNewLocalStoreRejectsBlankRoot(t *testing.T) {
	_, err := NewLocalStore("   ")

	if !errors.Is(err, ErrInvalidRoot) {
		t.Errorf("NewLocalStore() error = %v, want ErrInvalidRoot", err)
	}
}

func TestLocalStorePut(t *testing.T) {
	root := t.TempDir()
	store, err := NewLocalStore(root)
	if err != nil {
		t.Fatalf("NewLocalStore() error: %v", err)
	}
	content := []byte("hello cloud storage")
	sum := sha256.Sum256(content)
	wantChecksum := fmt.Sprintf("sha256:%x", sum)

	result, err := store.Put(
		context.Background(),
		"file-123",
		bytes.NewReader(content),
	)
	if err != nil {
		t.Fatalf("Put() error: %v", err)
	}

	if result.Size != int64(len(content)) {
		t.Errorf("Put() size = %d, want %d", result.Size, len(content))
	}
	if result.Checksum != wantChecksum {
		t.Errorf("Put() checksum = %q, want %q", result.Checksum, wantChecksum)
	}

	stored, err := os.ReadFile(filepath.Join(root, "file-123"))
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if !bytes.Equal(stored, content) {
		t.Errorf("stored content = %q, want %q", stored, content)
	}
}

func TestLocalStorePutRejectsInvalidKeys(t *testing.T) {
	store, err := NewLocalStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalStore() error: %v", err)
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
			_, err := store.Put(context.Background(), key, bytes.NewReader(nil))
			if !errors.Is(err, ErrInvalidKey) {
				t.Errorf("Put() error = %v, want ErrInvalidKey", err)
			}
		})
	}
}

func TestLocalStorePutRejectsNilSource(t *testing.T) {
	store, err := NewLocalStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalStore() error: %v", err)
	}

	_, err = store.Put(context.Background(), "file-123", nil)
	if !errors.Is(err, ErrInvalidSource) {
		t.Errorf("Put() error = %v, want ErrInvalidSource", err)
	}
}

func TestLocalStorePutDoesNotOverwrite(t *testing.T) {
	root := t.TempDir()
	store, err := NewLocalStore(root)
	if err != nil {
		t.Fatalf("NewLocalStore() error: %v", err)
	}

	if _, err := store.Put(
		context.Background(),
		"file-123",
		bytes.NewBufferString("original"),
	); err != nil {
		t.Fatalf("first Put() error: %v", err)
	}

	_, err = store.Put(
		context.Background(),
		"file-123",
		bytes.NewBufferString("replacement"),
	)
	if !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("second Put() error = %v, want ErrAlreadyExists", err)
	}

	stored, err := os.ReadFile(filepath.Join(root, "file-123"))
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if string(stored) != "original" {
		t.Errorf("stored content = %q, want %q", stored, "original")
	}
}

func TestLocalStorePutCancelled(t *testing.T) {
	root := t.TempDir()
	store, err := NewLocalStore(root)
	if err != nil {
		t.Fatalf("NewLocalStore() error: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = store.Put(ctx, "file-123", bytes.NewReader([]byte("content")))
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Put() error = %v, want context.Canceled", err)
	}
	assertDirectoryEmpty(t, root)
}

func TestLocalStorePutCleansUpAfterReadFailure(t *testing.T) {
	root := t.TempDir()
	store, err := NewLocalStore(root)
	if err != nil {
		t.Fatalf("NewLocalStore() error: %v", err)
	}
	wantErr := errors.New("source read failed")

	_, err = store.Put(context.Background(), "file-123", failingReader{err: wantErr})
	if !errors.Is(err, wantErr) {
		t.Errorf("Put() error = %v, want wrapped source error", err)
	}
	assertDirectoryEmpty(t, root)
}

type failingReader struct {
	err error
}

func (r failingReader) Read(p []byte) (int, error) {
	n := copy(p, "partial")
	return n, r.err
}

func assertDirectoryEmpty(t *testing.T, root string) {
	t.Helper()

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("ReadDir() error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("storage directory contains %d unexpected entries", len(entries))
	}
}

var _ io.Reader = failingReader{}
