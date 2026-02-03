package otp

import (
	"crypto/rand"
	"errors"
	"math/big"
	"sync"
	"time"
)

const GracePeriod = 5 * time.Minute

var (
	ErrNotGenerated = errors.New("OTP_NOT_GENERATED")
	ErrExpired      = errors.New("OTP_EXPIRED")
	ErrInvalid      = errors.New("INVALID_OTP")
)

type otpCode struct {
	code    string
	expires time.Time
}

type Service struct {
	mu       sync.Mutex
	codes    map[string]otpCode
	verified map[string]time.Time
}

func NewService() *Service {
	return &Service{
		codes:    make(map[string]otpCode),
		verified: make(map[string]time.Time),
	}
}

func (s *Service) Generate(username string) (string, error) {
	code, err := randomCode(6)
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	s.codes[username] = otpCode{code: code, expires: time.Now().Add(GracePeriod)}
	s.mu.Unlock()
	return code, nil
}

func (s *Service) Verify(username, code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.codes[username]
	if !ok {
		return ErrNotGenerated
	}
	if time.Now().After(entry.expires) {
		delete(s.codes, username)
		return ErrExpired
	}
	if entry.code != code {
		return ErrInvalid
	}
	delete(s.codes, username)
	s.verified[username] = time.Now().Add(GracePeriod)
	return nil
}

func (s *Service) Check(username string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	until, ok := s.verified[username]
	if !ok {
		return false
	}
	if time.Now().After(until) {
		delete(s.verified, username)
		return false
	}
	return true
}

func randomCode(length int) (string, error) {
	if length <= 0 {
		length = 6
	}
	max := big.NewInt(10)
	bytes := make([]byte, length)
	for i := range bytes {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		bytes[i] = byte('0' + n.Int64())
	}
	return string(bytes), nil
}
