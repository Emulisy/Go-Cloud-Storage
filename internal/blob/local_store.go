package blob

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
	"sync"
)

// LocalStore stores file content beneath one directory on local disk.
type LocalStore struct {
	root string
	mu   sync.Mutex
}

var _ Store = (*LocalStore)(nil)

func NewLocalStore(root string) (*LocalStore, error) {
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

	return &LocalStore{root: absRoot}, nil
}

func (s *LocalStore) Put(
	ctx context.Context,
	key string,
	source io.Reader,
) (PutResult, error) {
	if err := ctx.Err(); err != nil {
		return PutResult{}, err
	}
	if !validKey(key) {
		return PutResult{}, ErrInvalidKey
	}
	if source == nil {
		return PutResult{}, ErrInvalidSource
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	finalPath := filepath.Join(s.root, key)
	if _, err := os.Stat(finalPath); err == nil {
		return PutResult{}, ErrAlreadyExists
	} else if !errors.Is(err, os.ErrNotExist) {
		return PutResult{}, fmt.Errorf("stat blob destination: %w", err)
	}

	tempFile, err := os.CreateTemp(s.root, ".upload-*")
	if err != nil {
		return PutResult{}, fmt.Errorf("create temporary blob: %w", err)
	}

	tempPath := tempFile.Name()
	closed := false
	committed := false
	defer func() {
		if !closed {
			_ = tempFile.Close()
		}
		if !committed {
			_ = os.Remove(tempPath)
		}
	}()

	hasher := sha256.New()
	written, err := io.Copy(io.MultiWriter(tempFile, hasher), source)
	if err != nil {
		return PutResult{}, fmt.Errorf("copy blob content: %w", err)
	}

	if err := tempFile.Sync(); err != nil {
		return PutResult{}, fmt.Errorf("sync temporary blob: %w", err)
	}

	closeErr := tempFile.Close()
	closed = true
	if closeErr != nil {
		return PutResult{}, fmt.Errorf("close temporary blob: %w", closeErr)
	}

	if err := ctx.Err(); err != nil {
		return PutResult{}, err
	}

	if err := os.Rename(tempPath, finalPath); err != nil {
		return PutResult{}, fmt.Errorf("commit blob: %w", err)
	}
	committed = true

	return PutResult{
		Size:     written,
		Checksum: "sha256:" + hex.EncodeToString(hasher.Sum(nil)),
	}, nil
}

func (s *LocalStore) Delete(ctx context.Context, key string) error {
	// TODO 1: Return ctx.Err() when the request is already cancelled.
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	// TODO 2: Validate key with validKey and return ErrInvalidKey when invalid.
	if !validKey(key) {
		return ErrInvalidKey
	}
	// TODO 3: Lock the store so Delete cannot race with Put.
	s.mu.Lock()
	defer s.mu.Unlock()
	// TODO 4: Recheck ctx.Err() after acquiring the lock.
	if err := ctx.Err(); err != nil {
		return err
	}
	// TODO 5: Remove the file at filepath.Join(s.root, key).
	err := os.Remove(filepath.Join(s.root, key))
	if err != nil {
		// TODO 6: Translate os.ErrNotExist into ErrNotFound.
		if errors.Is(err, os.ErrNotExist) {
			return ErrNotFound
		}
		return fmt.Errorf("remove blob: %w", err)
	}
	return nil
}

func validKey(key string) bool {
	if key == "" {
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
