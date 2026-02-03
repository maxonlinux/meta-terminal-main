package impersonation

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"sync"
	"time"

	"github.com/maxonlinux/meta-terminal-go/internal/users"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

const ttl = 60 * time.Second

type record struct {
	userID types.UserID
	exp    time.Time
}

type Service struct {
	mu     sync.Mutex
	record map[string]record
	users  *users.Service
}

func NewService(userService *users.Service) *Service {
	return &Service{record: make(map[string]record), users: userService}
}

func (s *Service) Create(userID types.UserID) (string, error) {
	user, err := s.users.GetUserByID(userID)
	if err != nil || user == nil {
		return "", err
	}
	code, err := randomCode(24)
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	s.record[code] = record{userID: user.UserID, exp: time.Now().Add(ttl)}
	s.mu.Unlock()
	return code, nil
}

func (s *Service) Redeem(code string) (types.UserID, error) {
	s.mu.Lock()
	rec, ok := s.record[code]
	if !ok {
		s.mu.Unlock()
		return 0, errors.New("invalid code")
	}
	if time.Now().After(rec.exp) {
		delete(s.record, code)
		s.mu.Unlock()
		return 0, errors.New("expired code")
	}
	delete(s.record, code)
	s.mu.Unlock()
	return rec.userID, nil
}

func randomCode(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
