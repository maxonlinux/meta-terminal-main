package users

import (
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

type User struct {
	UserID       types.UserID `json:"userId" db:"id"`
	Username     string       `json:"username" db:"username"`
	PasswordHash string       `json:"-" db:"password_hash"`
}

type UserStore interface {
	CreateUser(username, passwordHash string) (types.UserID, error)
	GetUserByUsername(username string) (*User, error)
	GetUserByID(userID types.UserID) (*User, error)
	UserExists(username string) bool
}
