// Package blob defines backend-independent content results and errors.
package blob

import "errors"

var (
	// ErrInvalidKey indicates that a key violates the content adapter's naming rules.
	ErrInvalidKey = errors.New("invalid blob key")
	// ErrInvalidSource indicates that no content reader was supplied.
	ErrInvalidSource = errors.New("invalid blob source")
	// ErrAlreadyExists indicates that the destination key is already occupied.
	ErrAlreadyExists = errors.New("blob already exists")
	// ErrNotFound indicates that no content exists for the requested key.
	ErrNotFound = errors.New("blob not found")
)

// PutResult describes content after successful publication by a content store.
type PutResult struct {
	// Size is the number of content bytes written, excluding transport overhead.
	Size int64
	// Checksum identifies the algorithm and digest; the local adapter uses
	// "sha256:" followed by the lowercase hexadecimal digest.
	Checksum string
}
