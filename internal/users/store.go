package users

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/mattn/go-sqlite3"
	"github.com/maxonlinux/meta-terminal-go/pkg/snowflake"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/maxonlinux/meta-terminal-go/pkg/utils"
)

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(path string) (*SQLiteStore, error) {
	if err := os.MkdirAll("data", 0755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	db, err := sql.Open("sqlite3", path+"/users.db")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	if err := initDB(db); err != nil {
		return nil, fmt.Errorf("init database: %w", err)
	}

	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func initDB(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY,
			username TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			created_at INTEGER NOT NULL
		)
	`)
	return err
}

func (s *SQLiteStore) CreateUser(username, passwordHash string) (types.UserID, error) {
	userID := types.UserID(snowflake.Next())

	_, err := s.db.Exec(
		"INSERT INTO users (id, username, password_hash, created_at) VALUES (?, ?, ?, ?)",
		userID, username, passwordHash, utils.NowNano(),
	)
	if err != nil {
		return 0, fmt.Errorf("insert user: %w", err)
	}

	return userID, nil
}

func (s *SQLiteStore) GetUserByUsername(username string) (*User, error) {
	var user User
	err := s.db.QueryRow(
		"SELECT id, username, password_hash FROM users WHERE username = ?",
		username,
	).Scan(&user.UserID, &user.Username, &user.PasswordHash)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query user: %w", err)
	}

	return &user, nil
}

func (s *SQLiteStore) GetUserByID(userID types.UserID) (*User, error) {
	var user User
	err := s.db.QueryRow(
		"SELECT id, username, password_hash FROM users WHERE id = ?",
		userID,
	).Scan(&user.UserID, &user.Username, &user.PasswordHash)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query user: %w", err)
	}

	return &user, nil
}

func (s *SQLiteStore) UserExists(username string) bool {
	var exists int
	err := s.db.QueryRow(
		"SELECT 1 FROM users WHERE username = ? LIMIT 1",
		username,
	).Scan(&exists)

	return err == nil
}
