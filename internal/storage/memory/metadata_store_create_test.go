package memory

import (
	"context"
	"errors"
	"testing"

	"github.com/Emulisy/Go-Cloud-Storage/internal/files"
)

func TestMemoryStoreCreate(t *testing.T) {
	store := NewMetadataStore(nil)
	want := files.Metadata{
		ID:       "file-123",
		Name:     "notes.txt",
		Size:     128,
		Checksum: "sha256:example",
	}

	if err := store.Create(context.Background(), want); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	got, err := store.Get(context.Background(), want.ID)
	if err != nil {
		t.Fatalf("Get() error after Create(): %v", err)
	}

	if got != want {
		t.Errorf("Get() = %#v, want %#v", got, want)
	}
}

func TestMemoryStoreCreateDuplicateDoesNotOverwrite(t *testing.T) {
	original := files.Metadata{
		ID:   "file-123",
		Name: "original.txt",
		Size: 10,
	}
	store := NewMetadataStore([]files.Metadata{original})
	replacement := files.Metadata{
		ID:   original.ID,
		Name: "replacement.txt",
		Size: 20,
	}

	err := store.Create(context.Background(), replacement)
	if !errors.Is(err, files.ErrAlreadyExists) {
		t.Fatalf("Create() error = %v, want files.ErrAlreadyExists", err)
	}

	got, err := store.Get(context.Background(), original.ID)
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}

	if got != original {
		t.Errorf("duplicate Create() changed metadata: got %#v, want %#v", got, original)
	}
}

func TestMemoryStoreCreateRejectsInvalidMetadata(t *testing.T) {
	tests := []struct {
		name     string
		metadata files.Metadata
	}{
		{
			name:     "empty ID",
			metadata: files.Metadata{Name: "notes.txt"},
		},
		{
			name:     "whitespace ID",
			metadata: files.Metadata{ID: "   ", Name: "notes.txt"},
		},
		{
			name:     "empty name",
			metadata: files.Metadata{ID: "file-123"},
		},
		{
			name:     "whitespace name",
			metadata: files.Metadata{ID: "file-123", Name: "   "},
		},
		{
			name:     "negative size",
			metadata: files.Metadata{ID: "file-123", Name: "notes.txt", Size: -1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewMetadataStore(nil)

			err := store.Create(context.Background(), tt.metadata)

			if !errors.Is(err, files.ErrInvalidMetadata) {
				t.Errorf("Create() error = %v, want files.ErrInvalidMetadata", err)
			}
		})
	}
}

func TestMemoryStoreCreateCancelled(t *testing.T) {
	store := NewMetadataStore(nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := store.Create(ctx, files.Metadata{
		ID:   "file-123",
		Name: "notes.txt",
	})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("Create() error = %v, want context.Canceled", err)
	}
}
