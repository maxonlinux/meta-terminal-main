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
	if err := os.MkdirAll(path, 0o755); err != nil {
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

		CREATE TABLE IF NOT EXISTS user_profiles (
			user_id INTEGER PRIMARY KEY,
			email TEXT NOT NULL,
			phone TEXT NOT NULL,
			name TEXT,
			surname TEXT,
			is_active INTEGER NOT NULL
		)

		CREATE TABLE IF NOT EXISTS user_settings (
			user_id INTEGER PRIMARY KEY,
			is_2fa_enabled INTEGER NOT NULL,
			news_and_offers INTEGER NOT NULL,
			access_to_transaction_data INTEGER NOT NULL,
			access_to_geolocation INTEGER NOT NULL,
			preferences TEXT NOT NULL
		)

		CREATE TABLE IF NOT EXISTS user_addresses (
			user_id INTEGER PRIMARY KEY,
			country TEXT,
			city TEXT,
			address TEXT,
			zip TEXT
		)
	`)
	return err
}

func (s *SQLiteStore) CreateUser(username, passwordHash, email, phone string) (types.UserID, error) {
	userID := types.UserID(snowflake.Next())

	_, err := s.db.Exec(
		"INSERT INTO users (id, username, password_hash, created_at) VALUES (?, ?, ?, ?)",
		userID, username, passwordHash, utils.NowNano(),
	)
	if err != nil {
		return 0, fmt.Errorf("insert user: %w", err)
	}
	_, _ = s.db.Exec(
		"INSERT INTO user_profiles (user_id, email, phone, name, surname, is_active) VALUES (?, ?, ?, ?, ?, ?)",
		userID, email, phone, nil, nil, 0,
	)
	_, _ = s.db.Exec(
		"INSERT INTO user_settings (user_id, is_2fa_enabled, news_and_offers, access_to_transaction_data, access_to_geolocation, preferences) VALUES (?, ?, ?, ?, ?, ?)",
		userID, 0, 0, 0, 0, "{}",
	)
	_, _ = s.db.Exec(
		"INSERT INTO user_addresses (user_id, country, city, address, zip) VALUES (?, ?, ?, ?, ?)",
		userID, nil, nil, nil, nil,
	)

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

func (s *SQLiteStore) UpdatePassword(userID types.UserID, passwordHash string) error {
	_, err := s.db.Exec("UPDATE users SET password_hash = ? WHERE id = ?", passwordHash, userID)
	return err
}

func (s *SQLiteStore) GetProfile(userID types.UserID) (*UserProfile, error) {
	row := s.db.QueryRow(`select u.id, u.username, p.email, p.phone, p.name, p.surname, p.is_active from users u join user_profiles p on u.id = p.user_id where u.id = ?`, userID)
	var profile UserProfile
	var isActive int
	if err := row.Scan(&profile.UserID, &profile.Username, &profile.Email, &profile.Phone, &profile.Name, &profile.Surname, &isActive); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	profile.IsActive = isActive == 1
	return &profile, nil
}

func (s *SQLiteStore) UpdateProfile(userID types.UserID, name *string, surname *string) error {
	_, err := s.db.Exec(`update user_profiles set name = ?, surname = ? where user_id = ?`, name, surname, userID)
	return err
}

func (s *SQLiteStore) SetActive(userID types.UserID, active bool) error {
	flag := 0
	if active {
		flag = 1
	}
	_, err := s.db.Exec(`update user_profiles set is_active = ? where user_id = ?`, flag, userID)
	return err
}

func (s *SQLiteStore) GetSettings(userID types.UserID) (*UserSettings, error) {
	row := s.db.QueryRow(`select user_id, is_2fa_enabled, news_and_offers, access_to_transaction_data, access_to_geolocation, preferences from user_settings where user_id = ?`, userID)
	var settings UserSettings
	var is2fa, news, txData, geo int
	if err := row.Scan(&settings.UserID, &is2fa, &news, &txData, &geo, &settings.Preferences); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	settings.Is2FAEnabled = is2fa == 1
	settings.NewsAndOffers = news == 1
	settings.AccessToTransactionData = txData == 1
	settings.AccessToGeolocation = geo == 1
	return &settings, nil
}

func (s *SQLiteStore) UpdateSettings(userID types.UserID, settings UserSettings) error {
	_, err := s.db.Exec(`update user_settings set is_2fa_enabled = ?, news_and_offers = ?, access_to_transaction_data = ?, access_to_geolocation = ?, preferences = ? where user_id = ?`,
		boolToInt(settings.Is2FAEnabled),
		boolToInt(settings.NewsAndOffers),
		boolToInt(settings.AccessToTransactionData),
		boolToInt(settings.AccessToGeolocation),
		settings.Preferences,
		userID,
	)
	return err
}

func (s *SQLiteStore) GetAddress(userID types.UserID) (*UserAddress, error) {
	row := s.db.QueryRow(`select user_id, country, city, address, zip from user_addresses where user_id = ?`, userID)
	var addr UserAddress
	if err := row.Scan(&addr.UserID, &addr.Country, &addr.City, &addr.Address, &addr.Zip); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &addr, nil
}

func (s *SQLiteStore) UpdateAddress(userID types.UserID, address UserAddress) error {
	_, err := s.db.Exec(`update user_addresses set country = ?, city = ?, address = ?, zip = ? where user_id = ?`, address.Country, address.City, address.Address, address.Zip, userID)
	return err
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
