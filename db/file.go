package db

import (
	"database/sql"
	"fmt"
)

// update the file metadata to mysql
func OnFileUploadFinish(fileSHA string, fileName string, fileSize int64, fileAddr string) error {
	conn := DBConn()
	if conn == nil {
		return fmt.Errorf("insert file metadata: database is not initialized")
	}

	_, err := DBConn().Exec(
		`INSERT INTO tbl_file
		    (file_sha, file_name, file_size, file_addr, status)
		 VALUES (?, ?, ?, ?, 1)`,
		fileSHA,
		fileName,
		fileSize,
		fileAddr,
	)

	if err != nil {
		return fmt.Errorf("insert file metadata: %w", err)
	}

	return nil
}

// DeleteUploadedFileMeta removes a file row when a later upload step fails.
func DeleteUploadedFileMeta(fileSHA string) error {
	conn := DBConn()
	if conn == nil {
		return fmt.Errorf("delete file metadata: database is not initialized")
	}

	_, err := conn.Exec(`DELETE FROM tbl_file WHERE file_sha = ?`, fileSHA)
	if err != nil {
		return fmt.Errorf("delete file metadata: %w", err)
	}

	return nil
}

type TableFile struct {
	FileHash  string
	FileName  sql.NullString
	FileSize  sql.NullInt64
	FileAddr  sql.NullString
	CreatedAt sql.NullTime
}

// get file metadata from mysql
func GetFileMeta(filehash string) (*TableFile, error) {
	conn := DBConn()
	if conn == nil {
		return nil, fmt.Errorf("get file metadata: database is not initialized")
	}

	query := `
		SELECT file_sha, file_addr, file_name, file_size, create_at
		FROM tbl_file
		WHERE file_sha = ? AND status = 1
		LIMIT 1
	`

	tfile := &TableFile{}

	err := conn.QueryRow(query, filehash).Scan(
		&tfile.FileHash,
		&tfile.FileAddr,
		&tfile.FileName,
		&tfile.FileSize,
		&tfile.CreatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("get file metadata: %w", err)
	}

	return tfile, nil
}
