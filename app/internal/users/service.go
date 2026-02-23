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

func (s *Service) Register(username, password, email, phone string) (types.UserID, error) {
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

	if email == "" {
		return 0, errors.New("email is required")
	}
	if phone == "" {
		return 0, errors.New("phone is required")
	}

	if s.store.UserExists(username) {
		return 0, errors.New("username already exists")
	}

	hash, err := HashPassword(password)
	if err != nil {
		return 0, fmt.Errorf("failed to hash password: %w", err)
	}

	return s.store.CreateUser(username, string(hash), email, phone)
}

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
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

func (s *Service) GetProfile(userID types.UserID) (*UserProfile, error) {
	return s.store.GetProfile(userID)
}

func (s *Service) ListProfiles(limit, offset int, query string) ([]UserProfile, error) {
	return s.store.ListProfiles(limit, offset, query)
}

func (s *Service) UpdateProfile(userID types.UserID, name *string, surname *string) error {
	return s.store.UpdateProfile(userID, name, surname)
}

func (s *Service) SetActive(userID types.UserID, active bool) error {
	return s.store.SetActive(userID, active)
}

func (s *Service) UpdateLastLogin(userID types.UserID, lastLogin uint64) error {
	return s.store.UpdateLastLogin(userID, lastLogin)
}

func (s *Service) UpdateProfileDetails(userID types.UserID, email, phone string, name *string, surname *string) error {
	return s.store.UpdateProfileDetails(userID, email, phone, name, surname)
}

func (s *Service) GetSettings(userID types.UserID) (*UserSettings, error) {
	return s.store.GetSettings(userID)
}

func (s *Service) UpdateSettings(userID types.UserID, settings UserSettings) error {
	return s.store.UpdateSettings(userID, settings)
}

func (s *Service) GetAddress(userID types.UserID) (*UserAddress, error) {
	return s.store.GetAddress(userID)
}

func (s *Service) UpdateAddress(userID types.UserID, address UserAddress) error {
	return s.store.UpdateAddress(userID, address)
}

func (s *Service) UpdatePassword(userID types.UserID, passwordHash string) error {
	return s.store.UpdatePassword(userID, passwordHash)
}
