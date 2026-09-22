package db

import (
	"errors"
	"fmt"
	"strings"
	"github.com/go-sql-driver/mysql"
)

var ErrUsernameExists = errors.New("username already exists")

func UserSignUp(userName string, userPwd string) error {
	conn := DBConn()
	if conn == nil {
		return fmt.Errorf("user signup: database is not initialized")
	}

	userName = strings.TrimSpace(userName)

	if userName == "" || userPwd == "" {
		return fmt.Errorf("username and password cannot be empty")
	}

	if len(userName) > 64 {
		return fmt.Errorf("username is too long")
	}

	query := `
		INSERT INTO tbl_user (user_name, user_pwd)
		VALUES (?, ?)
	`

	res, err := conn.Exec(query, userName, userPwd)
	if err != nil {
		var mysqlErr *mysql.MySQLError

		// MySQL error 1062 means a UNIQUE constraint was violated.
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			return ErrUsernameExists
		}

		return fmt.Errorf("user signup: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check affected rows: %w", err)
	}

	if rowsAffected != 1 {
		return fmt.Errorf("user signup: expected 1 inserted row, got %d", rowsAffected)
	}

	return nil
}

func UserSignin(userName string, encPwd string) error {
}