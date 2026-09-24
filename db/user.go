package db

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
	"net/mail"
	"strings"
)

var ErrEmailExists = errors.New("email already exists")
var ErrInvalidCredentials = errors.New("invalid email or password")

func UserSignUp(userName string, email string, userPwd string) error {
	conn := DBConn()
	if conn == nil {
		return fmt.Errorf("user signup: database is not initialized")
	}

	userName = strings.TrimSpace(userName)
	email, err := NormalizeEmail(email)
	if err != nil {
		return err
	}

	if userName == "" || userPwd == "" {
		return fmt.Errorf("username and password cannot be empty")
	}

	if len(userName) > 64 {
		return fmt.Errorf("username is too long")
	}

	query := `
		INSERT INTO tbl_user (user_name, email, user_pwd)
		VALUES (?, ?, ?)
	`

	res, err := conn.Exec(query, userName, email, userPwd)
	if err != nil {
		var mysqlErr *mysql.MySQLError

		// MySQL error 1062 means a UNIQUE constraint was violated.
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			return ErrEmailExists
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

// NormalizeEmail accepts a plain address and normalizes it for account lookup.
func NormalizeEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	address, err := mail.ParseAddress(email)
	if err != nil || len(email) > 254 || address.Address != email || address.Name != "" {
		return "", fmt.Errorf("invalid email address")
	}
	return email, nil
}

func UserSignin(email string, userPwd string) (int64, error) {
	email, err := NormalizeEmail(email)
	if err != nil || userPwd == "" {
		return 0, ErrInvalidCredentials
	}
	conn := DBConn()
	if conn == nil {
		return 0, fmt.Errorf("user signin: database is not initialized")
	}
	var userID int64
	var storedHash string
	err = conn.QueryRow(`SELECT id, user_pwd FROM tbl_user WHERE email = ? AND status = 0 LIMIT 1`, email).Scan(&userID, &storedHash)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrInvalidCredentials
	}
	if err != nil {
		return 0, fmt.Errorf("user signin: %w", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(userPwd)); err != nil {
		return 0, ErrInvalidCredentials
	}
	return userID, nil
}

type UserInfo struct {
	Username   string
	Phone      string
	Email      string
	SignupAt   string
	LastActive string
	status     int
}

func GetUserInfo(userID int64) (*UserInfo, error) {
	conn := DBConn()
	if conn == nil {
		return nil, fmt.Errorf("user signup: database is not initialized")
	}

	if userID < 1 {
		return nil, ErrInvalidCredentials
	}

	user := &UserInfo{}

	query := `
		SELECT user_name, COALESCE(phone, ''), COALESCE(email, ''), signup_at, last_active
		FROM tbl_user
		WHERE id=? AND status=0
		LIMIT 1
	`

	err := conn.QueryRow(query, userID).Scan(
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
