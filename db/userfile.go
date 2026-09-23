package db

import (
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

var ErrUserFileNotFound = errors.New("user file not found")

type UserFile struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	FileHash    string `json:"fileHash"`
	FileName    string `json:"fileName"`
	FileSize    int64  `json:"fileSize"`
	UploadAt    string `json:"uploadAt"`
	LastUpdated string `json:"lastUpdated"`
}

// UserFileDownload contains the user-specific name and shared storage address.
type UserFileDownload struct {
	FileName string
	FileAddr string
}

// StoredFile describes shared content stored once in tbl_file.
type StoredFile struct {
	Hash string
	Size int64
	Addr string
}

// StoreUserFile creates the shared content row when needed and always creates a
// distinct user-file row. It reports whether the supplied content path was used.
func StoreUserFile(userName string, fileName string, file StoredFile) (bool, error) {
	conn := DBConn()
	if conn == nil {
		return false, fmt.Errorf("store user file: database is not initialized")
	}

	userName = strings.TrimSpace(userName)
	fileName = strings.TrimSpace(fileName)
	if userName == "" || len(userName) > 64 || fileName == "" || len(fileName) > 255 ||
		file.Hash == "" || file.Size < 0 || file.Addr == "" {
		return false, fmt.Errorf("store user file: invalid input")
	}

	tx, err := conn.Begin()
	if err != nil {
		return false, fmt.Errorf("begin storing user file: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.Exec(
		`INSERT INTO tbl_file (file_sha, file_size, file_addr, status)
		 VALUES (?, ?, ?, 1)
		 ON DUPLICATE KEY UPDATE file_sha = file_sha`,
		file.Hash,
		file.Size,
		file.Addr,
	)
	if err != nil {
		return false, fmt.Errorf("store shared file: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("check stored shared file: %w", err)
	}
	contentCreated := rowsAffected == 1

	if err := insertUserFile(tx, userName, fileName, file.Hash, file.Size); err != nil {
		return false, err
	}

	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit stored user file: %w", err)
	}

	return contentCreated, nil
}

// LinkUserFile creates a distinct user-file row for existing shared content.
func LinkUserFile(userName string, fileName string, fileHash string) error {
	conn := DBConn()
	if conn == nil {
		return fmt.Errorf("link user file: database is not initialized")
	}

	userName = strings.TrimSpace(userName)
	fileName = strings.TrimSpace(fileName)
	if userName == "" || len(userName) > 64 || fileName == "" || len(fileName) > 255 || fileHash == "" {
		return fmt.Errorf("link user file: invalid input")
	}

	tx, err := conn.Begin()
	if err != nil {
		return fmt.Errorf("begin linking user file: %w", err)
	}
	defer tx.Rollback()

	var fileSize int64
	err = tx.QueryRow(
		`SELECT file_size
		 FROM tbl_file
		 WHERE file_sha = ? AND status = 1
		 FOR UPDATE`,
		fileHash,
	).Scan(&fileSize)
	if err != nil {
		return fmt.Errorf("find shared file to link: %w", err)
	}

	if err := insertUserFile(tx, userName, fileName, fileHash, fileSize); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit linked user file: %w", err)
	}

	return nil
}

func insertUserFile(tx *sql.Tx, userName string, fileName string, fileHash string, fileSize int64) error {
	query := `
		INSERT INTO tbl_user_file
			(user_name, file_sha256, file_name, file_size, status)
		SELECT user_name, ?, ?, ?, 0
		FROM tbl_user
		WHERE user_name = ? AND status = 0
		LIMIT 1
	`

	result, err := tx.Exec(
		query,
		fileHash,
		fileName,
		fileSize,
		userName,
	)
	if err != nil {
		return fmt.Errorf("insert user file metadata: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check inserted user file metadata: %w", err)
	}
	if rowsAffected != 1 {
		return ErrInvalidCredentials
	}

	return nil
}

// GetStoredFile looks up active shared content by hash.
func GetStoredFile(fileHash string) (*StoredFile, error) {
	conn := DBConn()
	if conn == nil {
		return nil, fmt.Errorf("get stored file: database is not initialized")
	}

	stored := &StoredFile{}
	err := conn.QueryRow(
		`SELECT file_sha, file_size, file_addr
		 FROM tbl_file
		 WHERE file_sha = ? AND status = 1
		 LIMIT 1`,
		fileHash,
	).Scan(
		&stored.Hash,
		&stored.Size,
		&stored.Addr,
	)
	if err != nil {
		return nil, fmt.Errorf("get stored file: %w", err)
	}

	return stored, nil
}

// ListUserFiles returns active file records belonging to one user.
func ListUserFiles(userName string, page int, pageSize int) ([]UserFile, error) {
	if page < 1 || pageSize < 1 || page-1 > math.MaxInt/pageSize {
		return nil, fmt.Errorf("query user files: invalid page or page size")
	}

	conn := DBConn()
	if conn == nil {
		return nil, fmt.Errorf("query user files: database is not initialized")
	}

	userName = strings.TrimSpace(userName)
	if userName == "" || len(userName) > 64 {
		return nil, ErrInvalidCredentials
	}

	query := `
		SELECT
			f.id,
			u.user_name,
			f.file_sha256,
			f.file_name,
			f.file_size,
			f.upload_at,
			f.last_update
		FROM tbl_user_file AS f
		INNER JOIN tbl_user AS u
			ON f.user_name = u.user_name
		WHERE f.user_name = ?
			AND f.status = 0
			AND u.status = 0
		ORDER BY f.upload_at DESC, f.id DESC
		LIMIT ? OFFSET ?
	`

	rows, err := conn.Query(query, userName, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, fmt.Errorf("query user files: %w", err)
	}
	defer rows.Close()

	files := make([]UserFile, 0)

	for rows.Next() {
		var file UserFile
		var uploadAt, lastUpdated sql.NullTime

		if err := rows.Scan(
			&file.ID,
			&file.Username,
			&file.FileHash,
			&file.FileName,
			&file.FileSize,
			&uploadAt,
			&lastUpdated,
		); err != nil {
			return nil, fmt.Errorf("scan user file: %w", err)
		}

		if uploadAt.Valid {
			file.UploadAt = uploadAt.Time.Format(time.RFC3339)
		}
		if lastUpdated.Valid {
			file.LastUpdated = lastUpdated.Time.Format(time.RFC3339)
		}

		files = append(files, file)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user files: %w", err)
	}

	return files, nil
}

// GetUserFileDownload returns download details only when userName owns userFileID.
func GetUserFileDownload(userName string, userFileID int64) (*UserFileDownload, error) {
	userName = strings.TrimSpace(userName)
	if userName == "" || len(userName) > 64 || userFileID < 1 {
		return nil, ErrUserFileNotFound
	}

	conn := DBConn()
	if conn == nil {
		return nil, fmt.Errorf("get user file download: database is not initialized")
	}

	query := `
		SELECT uf.file_name, f.file_addr
		FROM tbl_user_file AS uf
		INNER JOIN tbl_file AS f
			ON f.file_sha = uf.file_sha256
		WHERE uf.id = ?
			AND uf.user_name = ?
			AND uf.status = 0
			AND f.status = 1
		LIMIT 1
	`

	download := &UserFileDownload{}
	err := conn.QueryRow(query, userFileID, userName).Scan(
		&download.FileName,
		&download.FileAddr,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserFileNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user file download: %w", err)
	}

	return download, nil
}

// DeleteUserFile removes one owned user-file row. If it was the final reference,
// it also removes the shared database row and returns its physical path for cleanup.
func DeleteUserFile(userName string, userFileID int64) (string, error) {
	userName = strings.TrimSpace(userName)
	if userName == "" || len(userName) > 64 || userFileID < 1 {
		return "", ErrUserFileNotFound
	}

	conn := DBConn()
	if conn == nil {
		return "", fmt.Errorf("delete user file: database is not initialized")
	}

	tx, err := conn.Begin()
	if err != nil {
		return "", fmt.Errorf("begin deleting user file: %w", err)
	}
	defer tx.Rollback()

	var fileHash string
	err = tx.QueryRow(
		`SELECT file_sha256
		 FROM tbl_user_file
		 WHERE id = ? AND user_name = ? AND status = 0
		 FOR UPDATE`,
		userFileID,
		userName,
	).Scan(&fileHash)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrUserFileNotFound
	}
	if err != nil {
		return "", fmt.Errorf("find user file to delete: %w", err)
	}

	var fileAddr string
	err = tx.QueryRow(
		`SELECT file_addr
		 FROM tbl_file
		 WHERE file_sha = ? AND status = 1
		 FOR UPDATE`,
		fileHash,
	).Scan(&fileAddr)
	if err != nil {
		return "", fmt.Errorf("find shared file to delete: %w", err)
	}

	result, err := tx.Exec(
		`DELETE FROM tbl_user_file WHERE id = ? AND user_name = ?`,
		userFileID,
		userName,
	)
	if err != nil {
		return "", fmt.Errorf("delete user file row: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return "", fmt.Errorf("check deleted user file row: %w", err)
	}
	if rowsAffected != 1 {
		return "", ErrUserFileNotFound
	}

	var referenceCount int
	if err := tx.QueryRow(
		`SELECT COUNT(*) FROM tbl_user_file WHERE file_sha256 = ?`,
		fileHash,
	).Scan(&referenceCount); err != nil {
		return "", fmt.Errorf("count shared file references: %w", err)
	}

	cleanupPath := ""
	if referenceCount == 0 {
		result, err = tx.Exec(`DELETE FROM tbl_file WHERE file_sha = ?`, fileHash)
		if err != nil {
			return "", fmt.Errorf("delete shared file row: %w", err)
		}
		rowsAffected, err = result.RowsAffected()
		if err != nil {
			return "", fmt.Errorf("check deleted shared file row: %w", err)
		}
		if rowsAffected != 1 {
			return "", fmt.Errorf("delete shared file row: unexpected row count %d", rowsAffected)
		}
		cleanupPath = fileAddr
	}

	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("commit deleted user file: %w", err)
	}

	return cleanupPath, nil
}

// RenameUserFile renames one active user-file row owned by userName.
func RenameUserFile(userName string, newFileName string, userFileID int64) error {
	conn := DBConn()
	if conn == nil {
		return fmt.Errorf("rename user file: database is not initialized")
	}

	userName = strings.TrimSpace(userName)
	newFileName = strings.TrimSpace(newFileName)
	if userName == "" || len(userName) > 64 || userFileID < 1 || newFileName == "" || len(newFileName) > 255 {
		return fmt.Errorf("rename user file: invalid input")
	}

	//start transaction
	tx, err := conn.Begin()
	if err != nil {
		return fmt.Errorf("begin rename user file: %w", err)
	}
	defer tx.Rollback()

	//first locate the file if it exist
	var currentFileName string
	err = tx.QueryRow(
		`SELECT file_name
		 FROM tbl_user_file
		 WHERE id = ? AND user_name = ? AND status = 0
		 FOR UPDATE`,
		userFileID,
		userName,
	).Scan(&currentFileName)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrUserFileNotFound
	}
	if err != nil {
		return fmt.Errorf("find user file to rename: %w", err)
	}

	//rename file
	if currentFileName != newFileName {
		result, err := tx.Exec(
			`UPDATE tbl_user_file
				SET file_name = ?
			 WHERE id = ? AND user_name = ? AND status = 0
			`,
			newFileName,
			userFileID,
			userName,
		)
		if err != nil {
			return fmt.Errorf("rename user file: %w", err)
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("check renamed user file: %w", err)
		}
		if rowsAffected != 1 {
			return ErrUserFileNotFound
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit renamed user file: %w", err)
	}

	return nil
}
