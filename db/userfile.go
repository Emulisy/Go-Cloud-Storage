package db

import (
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"
)

type UserFile struct {
	Username    string `json:"username"`
	FileHash    string `json:"fileHash"`
	FileName    string `json:"fileName"`
	FileSize    int64  `json:"fileSize"`
	UploadAt    string `json:"uploadAt"`
	LastUpdated string `json:"lastUpdated"`
}

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
