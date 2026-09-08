package blob

import (
	"context"
	"errors"
	"io"
)

var (
	ErrInvalidRoot   = errors.New("invalid blob storage root")
	ErrInvalidKey    = errors.New("invalid blob key")
	ErrInvalidSource = errors.New("invalid blob source")
	ErrAlreadyExists = errors.New("blob already exists")
	ErrNotFound      = errors.New("blob not found")
)

// PutResult describes content after it has been stored successfully.
type PutResult struct {
	Size     int64
	Checksum string
}

// Writer stores a byte stream under an application-generated object key.
type Writer interface {
	Put(ctx context.Context, key string, source io.Reader) (PutResult, error)
}

// Reader opens stored content as a stream. The caller must close it.
type Reader interface {
	Open(ctx context.Context, key string) (io.ReadCloser, error)
}

// Deleter removes content previously stored under a key.
type Deleter interface {
	Delete(ctx context.Context, key string) error
}

// Store combines the read, write, and delete operations implemented by LocalStore.
type Store interface {
	Writer
	Reader
	Deleter
}
