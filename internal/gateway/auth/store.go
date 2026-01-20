package auth

import (
	"bytes"
	"encoding/gob"
	"fmt"

	"github.com/maxonlinux/meta-terminal-go/pkg/persistence"
	"github.com/maxonlinux/meta-terminal-go/pkg/snowflake"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

type userStore struct {
	db *persistence.PebbleKV
}

func NewUserStore(db *persistence.PebbleKV) UserStore {
	return &userStore{db: db}
}

func (s *userStore) CreateUser(username, passwordHash string) (types.UserID, error) {
	userID := types.UserID(snowflake.Next())

	user := User{
		UserID:       userID,
		Username:     username,
		PasswordHash: passwordHash,
	}

	data, err := encodeUser(&user)
	if err != nil {
		return 0, fmt.Errorf("encode user: %w", err)
	}

	if err := s.db.Set(userKey(username), data); err != nil {
		return 0, fmt.Errorf("save user: %w", err)
	}

	if err := s.db.Set(userIDKey(userID), []byte(username)); err != nil {
		return 0, fmt.Errorf("save user index: %w", err)
	}

	return userID, nil
}

func (s *userStore) GetUserByUsername(username string) (*User, error) {
	data, err := s.db.Get(userKey(username))
	if err != nil {
		if err == persistence.ErrKeyNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get user: %w", err)
	}

	user, err := decodeUser(data)
	if err != nil {
		return nil, fmt.Errorf("decode user: %w", err)
	}

	return user, nil
}

func (s *userStore) GetUserByID(userID types.UserID) (*User, error) {
	username, err := s.db.Get(userIDKey(userID))
	if err != nil {
		if err == persistence.ErrKeyNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get user index: %w", err)
	}

	return s.GetUserByUsername(string(username))
}

func (s *userStore) UserExists(username string) bool {
	_, err := s.db.Get(userKey(username))
	return err == nil
}

func userKey(username string) []byte {
	return []byte(fmt.Sprintf("u:%s", username))
}

func userIDKey(userID types.UserID) []byte {
	return []byte(fmt.Sprintf("u:id:%016x", userID))
}

func encodeUser(u *User) ([]byte, error) {
	buf := bytes.Buffer{}
	if err := gob.NewEncoder(&buf).Encode(u); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decodeUser(data []byte) (*User, error) {
	var user User
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&user); err != nil {
		return nil, err
	}
	return &user, nil
}
