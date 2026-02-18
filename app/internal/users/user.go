package users

import (
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

type User struct {
	UserID       types.UserID `json:"userId" db:"id"`
	Username     string       `json:"username" db:"username"`
	PasswordHash string       `json:"-" db:"password_hash"`
}

type UserProfile struct {
	UserID   types.UserID
	Username string
	Email    string
	Phone    string
	Name     *string
	Surname  *string
	IsActive bool
}

type UserSettings struct {
	UserID                  types.UserID
	Is2FAEnabled            bool
	NewsAndOffers           bool
	AccessToTransactionData bool
	AccessToGeolocation     bool
	Preferences             string
}

type UserAddress struct {
	UserID  types.UserID
	Country *string
	City    *string
	Address *string
	Zip     *string
}

type UserStore interface {
	CreateUser(username, passwordHash, email, phone string) (types.UserID, error)
	GetUserByUsername(username string) (*User, error)
	GetUserByID(userID types.UserID) (*User, error)
	ListProfiles(limit, offset int, query string) ([]UserProfile, error)
	UserExists(username string) bool
	GetProfile(userID types.UserID) (*UserProfile, error)
	UpdateProfile(userID types.UserID, name *string, surname *string) error
	SetActive(userID types.UserID, active bool) error
	GetSettings(userID types.UserID) (*UserSettings, error)
	UpdateSettings(userID types.UserID, settings UserSettings) error
	GetAddress(userID types.UserID) (*UserAddress, error)
	UpdateAddress(userID types.UserID, address UserAddress) error
	UpdatePassword(userID types.UserID, passwordHash string) error
}
