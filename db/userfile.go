package db

import "fmt"

type UserFile struct {
	Username    string
	FileHash    string
	FileName    string
	FileSize    int64
	UploadAt    string
	LastUpdated string
}

func OnUserFileUploadFinish(
	userID int64,
	fileHash string,
	fileName string,
	fileSize int64,
) error {
	conn := DBConn()
	if conn == nil {
		return fmt.Errorf("insert user file: database is not initialized")
	}

	if userID <= 0 {
		return ErrInvalidCredentials
	}

	query := `
		INSERT INTO tbl_user_file
			(user_id, file_sha256, file_name, file_size, status)
		VALUES (?, ?, ?, ?, 0)
	`

	_, err := conn.Exec(
		query,
		userID,
		fileHash,
		fileName,
		fileSize,
	)

	if err != nil {
		return fmt.Errorf("insert user file metadata: %w", err)
	}

	return nil
}
