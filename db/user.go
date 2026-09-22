package db

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
	"strings"
)

var ErrUsernameExists = errors.New("username already exists")
var ErrInvalidCredentials = errors.New("invalid username or password")

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

func UserSignin(userName string, userPwd string) error {
	conn := DBConn()
	if conn == nil {
		return fmt.Errorf("user signup: database is not initialized")
	}

	if userName == "" || userPwd == "" {
		return ErrInvalidCredentials
	}

	if len(userName) > 64 {
		return ErrInvalidCredentials
	}

	query := `
		SELECT user_pwd
		FROM tbl_user
		WHERE user_name = ?
		LIMIT 1
	`

	// This variable receives the bcrypt hash stored in MySQL.
	var storedHash string

	err := conn.QueryRow(query, userName).Scan(&storedHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrInvalidCredentials
		}

		return fmt.Errorf("user signin: %w", err)
	}

	err = bcrypt.CompareHashAndPassword(
		[]byte(storedHash),
		[]byte(userPwd),
	)
	if err != nil {
		return ErrInvalidCredentials
	}
	return nil
}

type UserInfo struct {
	Username   string
	Phone      string
	Email      string
	SignupAt   string
	LastActive string
	status     int
}

func GetUserInfo(userName string) (*UserInfo, error) {
	conn := DBConn()
	if conn == nil {
		return nil, fmt.Errorf("user signup: database is not initialized")
	}

	if userName == "" {
		return nil, ErrInvalidCredentials
	}

	if len(userName) > 64 {
		return nil, ErrInvalidCredentials
	}

	user := &UserInfo{}

	query := `
		SELECT user_name, COALESCE(phone, ''), COALESCE(email, ''), signup_at, last_active
		FROM tbl_user
		WHERE user_name=? AND status=0
		LIMIT 1
	`

	err := conn.QueryRow(query, userName).Scan(
		&user.Username,
		&user.Phone,
		&user.Email,
		&user.SignupAt,
		&user.LastActive,
	)

	if err != nil {
		return nil, fmt.Errorf("get user info: %w", err)
	}

	return user, nil
}
