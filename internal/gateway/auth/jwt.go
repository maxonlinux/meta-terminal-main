package auth

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("token expired")
)

type Claims struct {
	UserID   types.UserID `json:"userId"`
	Username string       `json:"username"`
	jwt.RegisteredClaims
}

const (
	TokenExpiry  = 24 * time.Hour
	CookieName   = "token"
	CookieMaxAge = 86400
	CookiePath   = "/"
	JwtSecretEnv = "JWT_SECRET"
)

type JWTService struct {
	secret []byte
}

func NewJWTService() *JWTService {
	secret := []byte("dev-secret-change-in-production")
	if envSecret := os.Getenv(JwtSecretEnv); envSecret != "" {
		secret = []byte(envSecret)
	}
	return &JWTService{secret: secret}
}

func (s *JWTService) CreateToken(userID types.UserID, username string) (string, error) {
	claims := &Claims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(TokenExpiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "meta-terminal-go",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

func (s *JWTService) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.secret, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}
