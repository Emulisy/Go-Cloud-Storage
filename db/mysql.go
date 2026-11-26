package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/go-sql-driver/mysql"
)

var db *sql.DB

func InitDB() error {
	cfg := mysql.NewConfig()

	cfg.User = os.Getenv("MYSQL_USER")
	cfg.Passwd = os.Getenv("MYSQL_PASSWORD")
	cfg.Net = "tcp"
	cfg.Addr = os.Getenv("MYSQL_ADDR")
	if cfg.Addr == "" {
		cfg.Addr = "127.0.0.1:3306"
	}
	cfg.DBName = os.Getenv("MYSQL_DATABASE")
	cfg.ParseTime = true
	cfg.Loc = time.UTC

	if cfg.User == "" || cfg.Passwd == "" || cfg.DBName == "" {
		return fmt.Errorf("missing MySQL environment variables")
	}

	var err error

	db, err = sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return fmt.Errorf("failed to open MySQL: %w", err)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)

	if err = db.Ping(); err != nil {
		db.Close()
		db = nil
		return fmt.Errorf("failed to connect to MySQL: %w", err)
	}

	fmt.Println("Successfully connected to MySQL")

	return nil
}

// CheckStoredObjectAddresses keeps the app offline until old objects are migrated.
func CheckStoredObjectAddresses() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var needsMigration bool
	err := DBConn().QueryRowContext(ctx, `SELECT EXISTS(
		SELECT 1 FROM tbl_file
		WHERE LOCATE('://', file_addr) > 0
		  AND CAST(file_addr AS BINARY) NOT LIKE 's3://%'
	)`).Scan(&needsMigration)
	if err != nil {
		return fmt.Errorf("check stored object addresses: %w", err)
	}
	if needsMigration {
		return errors.New("database contains object addresses that must be migrated to MinIO before starting the app")
	}
	return nil
}

// DBConn returns the shared database connection pool.
func DBConn() *sql.DB {
	return db
}
