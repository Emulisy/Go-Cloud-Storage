package files

import (
	"context"
	"errors"
	"testing"
)

func TestMemoryStoreGet(t *testing.T) {
	want := Metadata{
		ID:       "file-123",
		Name:     "notes.txt",
		Size:     128,
		Checksum: "sha256:example",
	}
	store := NewMemoryStore([]Metadata{want})

	got, err := store.Get(context.Background(), want.ID)
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}

	if got != want {
		t.Errorf("Get() = %#v, want %#v", got, want)
	}
}

func TestMemoryStoreGetNotFound(t *testing.T) {
	store := NewMemoryStore(nil)

	_, err := store.Get(context.Background(), "missing")

	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Get() error = %v, want ErrNotFound", err)
	}
}

func TestMemoryStoreGetCancelled(t *testing.T) {
	store := NewMemoryStore([]Metadata{{
		ID:   "file-123",
		Name: "notes.txt",
	}})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := store.Get(ctx, "file-123")

	if !errors.Is(err, context.Canceled) {
		t.Errorf("Get() error = %v, want context.Canceled", err)
	}
}
