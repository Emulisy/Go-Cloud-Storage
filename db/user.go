package db

import (
	"database/sql"
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
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
		SELECT user_name, email, signup_at, last_active
		FROM tbl_user
		WHERE id=? AND status=0
		LIMIT 1
	`

	err := conn.QueryRow(query, userID).Scan(
		&user.Username,
		&user.Email,
		&user.SignupAt,
		&user.LastActive,
	)

	if err != nil {
		return nil, fmt.Errorf("get user info: %w", err)
	}

	return user, nil
}

// UpdateUserName changes the non-unique display name of an active user.
func UpdateUserName(userID int64, userName string) error {
	userName = strings.TrimSpace(userName)
	if len(userName) < 3 || len(userName) > 64 {
		return fmt.Errorf("username must be between 3 and 64 bytes")
	}
	return updateActiveUser(userID, func(tx *sql.Tx) error {
		_, err := tx.Exec(`UPDATE tbl_user SET user_name = ? WHERE id = ?`, userName, userID)
		return err
	})
}

// UpdateUserPwd verifies the current password and stores a bcrypt hash of the new one.
// Both password arguments are plaintext; callers must never pass a stored hash.
func UpdateUserPwd(userID int64, currentPwd string, newPwd string) error {
	if len(currentPwd) < 5 || len(currentPwd) > 72 {
		return ErrInvalidCredentials
	}
	if len(newPwd) < 5 || len(newPwd) > 72 {
		return fmt.Errorf("password must be between 5 and 72 bytes")
	}
	return updateActiveUser(userID, func(tx *sql.Tx) error {
		var storedHash string
		if err := tx.QueryRow(`SELECT user_pwd FROM tbl_user WHERE id = ?`, userID).Scan(&storedHash); err != nil {
			return err
		}
		if err := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(currentPwd)); err != nil {
			return ErrInvalidCredentials
		}
		hashedPwd, err := bcrypt.GenerateFromPassword([]byte(newPwd), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		_, err = tx.Exec(`UPDATE tbl_user SET user_pwd = ? WHERE id = ?`, string(hashedPwd), userID)
		return err
	})
}

// UpdateUserEmail changes the unique sign-in email and clears verification when changed.
func UpdateUserEmail(userID int64, email string) error {
	email, err := NormalizeEmail(email)
	if err != nil {
		return err
	}
	return updateActiveUser(userID, func(tx *sql.Tx) error {
		var currentEmail string
		if err := tx.QueryRow(`SELECT email FROM tbl_user WHERE id = ?`, userID).Scan(&currentEmail); err != nil {
			return err
		}
		if currentEmail == email {
			return nil
		}
		_, err := tx.Exec(`UPDATE tbl_user SET email = ?, email_validated = FALSE WHERE id = ?`, email, userID)
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			return ErrEmailExists
		}
		return err
	})
}

// updateActiveUser locks the account so concurrent changes cannot bypass validation.
// Checking existence separately also allows unchanged values to succeed.
func updateActiveUser(userID int64, update func(*sql.Tx) error) error {
	if userID < 1 {
		return ErrInvalidCredentials
	}
	conn := DBConn()
	if conn == nil {
		return fmt.Errorf("update user: database is not initialized")
	}
	tx, err := conn.Begin()
	if err != nil {
		return fmt.Errorf("begin user update: %w", err)
	}
	defer tx.Rollback()
	var id int64
	err = tx.QueryRow(`SELECT id FROM tbl_user WHERE id = ? AND status = 0 FOR UPDATE`, userID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrInvalidCredentials
	}
	if err != nil {
		return fmt.Errorf("find user to update: %w", err)
	}
	if err := update(tx); err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit user update: %w", err)
	}
	return nil
}
