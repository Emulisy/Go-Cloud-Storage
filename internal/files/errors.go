package files

import "errors"

// Metadata adapters return these sentinels directly or wrap them.
// Callers should classify errors with errors.Is.
var (
	// ErrNotFound indicates that no metadata record exists for the requested ID.
	ErrNotFound = errors.New("file metadata not found")
	// ErrAlreadyExists indicates that Create would replace an existing record.
	ErrAlreadyExists = errors.New("file metadata already exists")
	// ErrInvalidMetadata indicates a blank ID/name or a negative content size.
	ErrInvalidMetadata = errors.New("invalid file metadata")
)
