package files

import (
	"context"
	"errors"
	"strings"
	"sync"
)

var (
	ErrNotFound        = errors.New("file metadata not found")
	ErrAlreadyExists   = errors.New("file metadata already exists")
	ErrInvalidMetadata = errors.New("invalid file metadata")
)

// Reader is the behavior required by code that retrieves file metadata.
type Reader interface {
	Get(ctx context.Context, id string) (Metadata, error)
}

// Writer is the behavior required by code that creates file metadata.
type Writer interface {
	Create(ctx context.Context, metadata Metadata) error
}

// Store combines the read and write behaviors implemented by MemoryStore.
type Store interface {
	Reader
	Writer
}

// MemoryStore keeps file metadata in process memory.
//
// The mutex is required because HTTP handlers may call Get concurrently. A
// later lesson will add write operations that use the same map.
type MemoryStore struct {
	mu    sync.RWMutex
	files map[string]Metadata
}

var _ Store = (*MemoryStore)(nil)

func NewMemoryStore(initial []Metadata) *MemoryStore {
	files := make(map[string]Metadata, len(initial))
	for _, file := range initial {
		files[file.ID] = file
	}

	return &MemoryStore{
		files: files,
	}
}

func (s *MemoryStore) Get(ctx context.Context, id string) (Metadata, error) {
	if err := ctx.Err(); err != nil {
		return Metadata{}, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	file, ok := s.files[id]
	if !ok {
		return Metadata{}, ErrNotFound
	}

	return file, nil
}

func (s *MemoryStore) Create(ctx context.Context, metadata Metadata) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if strings.TrimSpace(metadata.ID) == "" {
		return ErrInvalidMetadata
	}

	if strings.TrimSpace(metadata.Name) == "" {
		return ErrInvalidMetadata
	}
	if metadata.Size < 0 {
		return ErrInvalidMetadata
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.files[metadata.ID]; exists {
		return ErrAlreadyExists
	}
	s.files[metadata.ID] = metadata
	return nil
}
