// Package files defines file metadata and backend-independent metadata errors.
package files

import "strings"

// Metadata describes a stored file without containing its bytes.
//
// Transport and persistence adapters own their representations, so this type
// carries no JSON or database mapping tags.
type Metadata struct {
	// ID associates this record with its content key.
	ID string
	// Name is a display name, not a filesystem path or storage key.
	Name string
	// Size is the measured content length in bytes.
	Size int64
	// Checksum is an optional algorithm-prefixed content digest.
	Checksum string
}

// Validate requires non-blank ID and Name values and a non-negative Size.
// It does not normalize fields or validate ID and checksum formats.
func (m Metadata) Validate() error {
	if strings.TrimSpace(m.ID) == "" || strings.TrimSpace(m.Name) == "" || m.Size < 0 {
		return ErrInvalidMetadata
	}
	return nil
}
