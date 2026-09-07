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
	// TODO 1: Allocate the map with make.
	files := make(map[string]Metadata)
	// TODO 2: Copy every initial item into the map, keyed by its ID.
	for _, file := range initial {
		files[file.ID] = file
	}
	// TODO 3: Return the initialized store.
	return &MemoryStore{
		files: files,
	}
}

func (s *MemoryStore) Get(ctx context.Context, id string) (Metadata, error) {
	// TODO 4: Return ctx.Err() if the context is already cancelled.
	err := ctx.Err()
	if err != nil {
		return Metadata{}, err
	}
	// TODO 5: Acquire a read lock and defer its release.
	s.mu.RLock()
	defer s.mu.RUnlock()
	// TODO 6: Look up id in the map using Go's value, ok form.
	file, ok := s.files[id]
	if !ok {
		// TODO 7: Return ErrNotFound when the ID does not exist.
		return Metadata{}, ErrNotFound
	}
	// TODO 8: Return the metadata when it exists.
	return file, nil




































	
}
