// Package local stores file content on the local filesystem.
package local

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Emulisy/Go-Cloud-Storage/internal/blob"
)

// ErrInvalidRoot indicates that the local storage directory is not configured.
var ErrInvalidRoot = errors.New("invalid blob storage root")

// BlobStore stores file content beneath one directory on local disk.
// It is safe for concurrent use. The filesystem must support hard links so
// completed uploads can be published atomically without replacing existing files.
// Construct stores with NewBlobStore; the zero value is not usable. The root must
// remain under trusted application control throughout the store's lifetime.
type BlobStore struct {
	root string
}

// NewBlobStore resolves root to an absolute path and creates missing directories.
// Relative roots are resolved against the process working directory. Existing
// directory permissions are preserved. Blank roots return ErrInvalidRoot;
// filesystem failures are wrapped. Hard-link support is exercised during Put.
func NewBlobStore(root string) (*BlobStore, error) {
	if strings.TrimSpace(root) == "" {
		return nil, ErrInvalidRoot
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve blob storage root: %w", err)
	}

	if err := os.MkdirAll(absRoot, 0o700); err != nil {
		return nil, fmt.Errorf("create blob storage root: %w", err)
	}

	return &BlobStore{root: absRoot}, nil
}

// Put copies source into a temporary file, measures and hashes its bytes, then
// publishes it under key without replacing existing content. The caller owns
// source; Put never closes it.
//
// Keys must be non-empty and contain only ASCII letters, digits, '-' or '_';
// platform-reserved names are also rejected. Invalid keys, nil readers, and
// occupied keys return blob.ErrInvalidKey, blob.ErrInvalidSource, and
// blob.ErrAlreadyExists, respectively.
//
// Cancellation is checked between reads and before publication. Temporary-file
// cleanup is best effort. File content is synced, but the parent directory is not,
// so publication does not promise full durability across a system crash.
func (s *BlobStore) Put(
	ctx context.Context,
	key string,
	source io.Reader,
) (blob.PutResult, error) {
	if err := ctx.Err(); err != nil {
		return blob.PutResult{}, err
	}
	if !validKey(key) {
		return blob.PutResult{}, blob.ErrInvalidKey
	}
	if source == nil {
		return blob.PutResult{}, blob.ErrInvalidSource
	}

	// This early check avoids unnecessary copying; Link below resolves races.
	finalPath := filepath.Join(s.root, key)
	if _, err := os.Lstat(finalPath); err == nil {
		return blob.PutResult{}, blob.ErrAlreadyExists
	} else if !errors.Is(err, os.ErrNotExist) {
		return blob.PutResult{}, fmt.Errorf("stat blob destination: %w", err)
	}

	// Keeping both names in one directory avoids cross-filesystem publication.
	tempFile, err := os.CreateTemp(s.root, ".upload-*")
	if err != nil {
		return blob.PutResult{}, fmt.Errorf("create temporary blob: %w", err)
	}
	defer func() {
		_ = tempFile.Close()
		_ = os.Remove(tempFile.Name())
	}()

	hasher := sha256.New()
	written, err := io.Copy(io.MultiWriter(tempFile, hasher), contextReader{
		ctx:    ctx,
		source: source,
	})
	if err != nil {
		return blob.PutResult{}, fmt.Errorf("copy blob content: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return blob.PutResult{}, err
	}

	if err := tempFile.Sync(); err != nil {
		return blob.PutResult{}, fmt.Errorf("sync temporary blob: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return blob.PutResult{}, fmt.Errorf("close temporary blob: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return blob.PutResult{}, err
	}

	// Unlike Rename, Link cannot overwrite a key created by another store instance.
	if err := os.Link(tempFile.Name(), finalPath); err != nil {
		if errors.Is(err, os.ErrExist) {
			return blob.PutResult{}, blob.ErrAlreadyExists
		}
		return blob.PutResult{}, fmt.Errorf("commit blob: %w", err)
	}

	return blob.PutResult{
		Size:     written,
		Checksum: "sha256:" + hex.EncodeToString(hasher.Sum(nil)),
	}, nil
}

// Open opens a regular file for key or returns blob.ErrNotFound when absent.
// Invalid keys return blob.ErrInvalidKey. The caller must close the returned
// stream. Context cancellation is checked at entry, not on subsequent reads.
func (s *BlobStore) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !validKey(key) {
		return nil, blob.ErrInvalidKey
	}

	file, err := os.Open(filepath.Join(s.root, key))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, blob.ErrNotFound
		}
		return nil, fmt.Errorf("open blob: %w", err)
	}

	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("stat blob: %w", err)
	}
	if !info.Mode().IsRegular() {
		_ = file.Close()
		return nil, fmt.Errorf("blob %q is not a regular file", key)
	}

	return file, nil
}

// Delete removes the directory entry for key. It returns blob.ErrNotFound when
// absent and blob.ErrInvalidKey for an invalid key; other errors retain their
// filesystem cause. It does not remove metadata or manage already-open readers.
func (s *BlobStore) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !validKey(key) {
		return blob.ErrInvalidKey
	}

	if err := os.Remove(filepath.Join(s.root, key)); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return blob.ErrNotFound
		}
		return fmt.Errorf("remove blob: %w", err)
	}

	return nil
}

func validKey(key string) bool {
	// IsLocal also rejects reserved device names on Windows.
	if !filepath.IsLocal(key) {
		return false
	}

	for _, r := range key {
		isLower := r >= 'a' && r <= 'z'
		isUpper := r >= 'A' && r <= 'Z'
		isDigit := r >= '0' && r <= '9'
		if !isLower && !isUpper && !isDigit && r != '-' && r != '_' {
			return false
		}
	}

	return true
}

// contextReader checks cancellation between reads; the source controls any
// blocking read already in progress.
type contextReader struct {
	ctx    context.Context
	source io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.source.Read(p)
}
