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

// insert user file connection
func OnUserFileUploadFinish(
	userName string,
	fileHash string,
	fileName string,
	fileSize int64,
) error {
	conn := DBConn()
	if conn == nil {
		return fmt.Errorf("insert user file: database is not initialized")
	}

	userName = strings.TrimSpace(userName)
	if userName == "" || len(userName) > 64 {
		return ErrInvalidCredentials
	}

	query := `
		INSERT INTO tbl_user_file
			(user_name, file_sha256, file_name, file_size, status)
		SELECT user_name, ?, ?, ?, 0
		FROM tbl_user
		WHERE user_name = ? AND status = 0
		LIMIT 1
	`

	result, err := conn.Exec(
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

// QueryUserFileMetas returns active file records belonging to one user.
func QueryUserFileMetas(userName string, page int, pageSize int) ([]UserFile, error) {
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

// UpdateUserFileMeta renames one active user-file row owned by userName.
func UpdateUserFileMeta(userName string, newFileName string, userFileID int64) error {
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
