package users

import (
	"errors"
	"fmt"

	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	store UserStore
}

func NewService(store UserStore) *Service {
	return &Service{store: store}
}

func (s *Service) Register(username, password string) (types.UserID, error) {
	if username == "" {
		return 0, errors.New("username is required")
	}
	if len(username) < 3 {
		return 0, errors.New("username must be at least 3 characters")
	}
	if password == "" {
		return 0, errors.New("password is required")
	}
	if len(password) < 8 {
		return 0, errors.New("password must be at least 8 characters")
	}

	if s.store.UserExists(username) {
		return 0, errors.New("username already exists")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return 0, fmt.Errorf("failed to hash password: %w", err)
	}

	return s.store.CreateUser(username, string(hash))
}

func (s *Service) ValidatePassword(user *User, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	return err == nil
}

func (s *Service) GetUserByUsername(username string) (*User, error) {
	return s.store.GetUserByUsername(username)
}

func (s *Service) GetUserByID(userID types.UserID) (*User, error) {
	return s.store.GetUserByID(userID)
}
