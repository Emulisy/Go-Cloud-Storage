package db

import (
	"database/sql"
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

// DBConn returns the shared database connection pool.
func DBConn() *sql.DB {
	return db
}
