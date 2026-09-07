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
