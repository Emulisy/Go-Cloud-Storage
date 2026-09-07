package files

import (
	"context"
	"errors"
	"sync"
)

var ErrNotFound = errors.New("file metadata not found")

// Reader is the behavior required by code that retrieves file metadata.
type Reader interface {
	Get(ctx context.Context, id string) (Metadata, error)
}

// MemoryStore keeps file metadata in process memory.
//
// The mutex is required because HTTP handlers may call Get concurrently. A
// later lesson will add write operations that use the same map.
type MemoryStore struct {
	mu    sync.RWMutex
	files map[string]Metadata
}

var _ Reader = (*MemoryStore)(nil)

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
