// Package memory provides process-local, volatile file metadata storage.
// Stores do not persist records or reconstruct them from content on disk.
package memory

import (
	"context"
	"sync"

	"github.com/Emulisy/Go-Cloud-Storage/internal/files"
)

// MetadataStore keeps metadata in process memory. It is safe for concurrent use.
// Its zero value is ready to use. A MetadataStore must not be copied after use.
type MetadataStore struct {
	mu      sync.RWMutex
	records map[string]files.Metadata
}

// NewMetadataStore copies optional seed records into a new store.
// Seeds are trusted fixtures: they are not validated, and later records with a
// duplicate ID replace earlier ones. Use Create for validated runtime writes.
func NewMetadataStore(initial []files.Metadata) *MetadataStore {
	records := make(map[string]files.Metadata, len(initial))
	for _, file := range initial {
		records[file.ID] = file
	}

	return &MetadataStore{records: records}
}

// Get returns a metadata value or files.ErrNotFound if id is absent.
// Cancellation is checked before and after acquiring the read lock; lock waiting
// itself is not context-aware.
func (s *MetadataStore) Get(ctx context.Context, id string) (files.Metadata, error) {
	if err := ctx.Err(); err != nil {
		return files.Metadata{}, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if err := ctx.Err(); err != nil {
		return files.Metadata{}, err
	}

	file, ok := s.records[id]
	if !ok {
		return files.Metadata{}, files.ErrNotFound
	}

	return file, nil
}

// Create validates metadata and inserts it without replacing an existing ID.
// Validation failures return files.ErrInvalidMetadata; duplicate IDs return
// files.ErrAlreadyExists. A returned error means no record was inserted.
func (s *MetadataStore) Create(ctx context.Context, metadata files.Metadata) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if err := metadata.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return err
	}

	// Checking and inserting under one lock makes uniqueness atomic to callers.
	if _, exists := s.records[metadata.ID]; exists {
		return files.ErrAlreadyExists
	}
	if s.records == nil {
		s.records = make(map[string]files.Metadata)
	}
	s.records[metadata.ID] = metadata
	return nil
}
