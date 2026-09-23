package meta

import (
	"errors"
	"sync"

	"goCloudStorage/db"
)

// file metadata struct
type FileMeta struct {
	FileSha256 string
	Location   string
	FileSize   int64
}

var fileMetas map[string]FileMeta
var fileMetasMu sync.RWMutex

func init() {
	fileMetas = make(map[string]FileMeta)
}

// update or upload new file metadata
func UpdateFileMeta(fm FileMeta) {
	fileMetasMu.Lock()
	defer fileMetasMu.Unlock()

	fileMetas[fm.FileSha256] = fm
}

// GetFileMeta returns file metadata using its SHA-256 digest.
func GetFileMeta(sha256 string) (FileMeta, error) {
	fileMetasMu.RLock()
	defer fileMetasMu.RUnlock()

	fm, exists := fileMetas[sha256]

	if !exists {
		return FileMeta{}, errors.New("file not found")
	}

	return fm, nil
}

// GetFileMetaDB gets file metadata from MySQL.
func GetFileMetaDB(sha256 string) (FileMeta, error) {
	storedFile, err := db.GetStoredFile(sha256)
	if err != nil {
		return FileMeta{}, err
	}

	return FileMeta{
		FileSha256: storedFile.Hash,
		FileSize:   storedFile.Size,
		Location:   storedFile.Addr,
	}, nil
}

// GetAllFileMetas returns a snapshot of all stored file metadata.
func GetAllFileMetas() []FileMeta {
	fileMetasMu.RLock()
	defer fileMetasMu.RUnlock()

	result := make([]FileMeta, 0, len(fileMetas))

	for _, fm := range fileMetas {
		result = append(result, fm)
	}

	return result
}

// DeleteFileMeta removes metadata using its SHA-256 digest.
func DeleteFileMeta(sha256 string) error {
	fileMetasMu.Lock()
	defer fileMetasMu.Unlock()

	if _, exists := fileMetas[sha256]; !exists {
		return errors.New("file not found")
	}

	delete(fileMetas, sha256)

	return nil
}
