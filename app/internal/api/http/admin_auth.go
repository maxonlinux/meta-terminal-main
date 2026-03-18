package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v5"
	"github.com/maxonlinux/meta-terminal-go/pkg/config"
	"golang.org/x/crypto/bcrypt"
)

type AdminAuthHandler struct{}

type AdminAuthStatusResponse struct {
	Initialized bool `json:"initialized"`
}

type AdminSetupRequest struct {
	Password string `json:"password"`
}

type AdminLoginRequest struct {
	Password string `json:"password"`
}

type AdminClaims struct {
	jwt.RegisteredClaims
}

func (h *AdminAuthHandler) Status(c *echo.Context) error {
	// Returns whether the admin password has been initialized.
	initialized, err := isAdminPasswordInitialized()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to check status"})
	}
	return c.JSON(http.StatusOK, AdminAuthStatusResponse{Initialized: initialized})
}

func (h *AdminAuthHandler) Setup(c *echo.Context) error {
	// Creates the admin password if it does not exist yet.
	var req AdminSetupRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	if len(req.Password) < 6 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "password must be at least 6 characters"})
	}

	initialized, err := isAdminPasswordInitialized()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to check status"})
	}
	if initialized {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "admin password already set"})
	}

	if err := saveAdminPassword(req.Password); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to save password"})
	}

	return c.NoContent(http.StatusOK)
}

func (h *AdminAuthHandler) Login(c *echo.Context) error {
	// Validates the password and issues an admin session cookie.
	var req AdminLoginRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	if err := verifyAdminPassword(req.Password); err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid password"})
	}

	token, err := createAdminToken()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create token"})
	}

	c.SetCookie(&http.Cookie{
		Name:     adminCookieName(),
		Value:    token,
		Path:     adminCookiePath(),
		HttpOnly: true,
		Secure:   isSecureRequest(c.Request()),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   adminCookieMaxAge(),
	})

	return c.JSON(http.StatusOK, map[string]bool{"success": true})
}

func (h *AdminAuthHandler) Logout(c *echo.Context) error {
	// Clears the admin session cookie.
	c.SetCookie(&http.Cookie{
		Name:     adminCookieName(),
		Value:    "",
		Path:     adminCookiePath(),
		HttpOnly: true,
		Secure:   isSecureRequest(c.Request()),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	return c.JSON(http.StatusOK, map[string]bool{"success": true})
}

func (h *AdminAuthHandler) Middleware() echo.MiddlewareFunc {
	return h.requireAdminCookie()
}

func (h *AdminAuthHandler) requireAdminCookie() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			cookie, err := c.Request().Cookie(adminCookieName())
			if err != nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
			}

			secret, err := adminJWTSecret()
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "admin auth secret not configured"})
			}
			claims := &AdminClaims{}
			_, err = jwt.ParseWithClaims(cookie.Value, claims, func(token *jwt.Token) (interface{}, error) {
				return []byte(secret), nil
			})
			if err != nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid token"})
			}

			c.Set("admin", claims)
			return next(c)
		}
	}
}

func isAdminPasswordInitialized() (bool, error) {
	passwordFile := adminPasswordPath()
	dir := filepath.Dir(passwordFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return false, err
	}
	_, err := os.Stat(passwordFile)
	return !os.IsNotExist(err), nil
}

func saveAdminPassword(password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	passwordFile := adminPasswordPath()
	dir := filepath.Dir(passwordFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(passwordFile, hash, 0600)
}

func verifyAdminPassword(password string) error {
	passwordFile := adminPasswordPath()
	hash, err := os.ReadFile(passwordFile)
	if err != nil {
		return err
	}
	return bcrypt.CompareHashAndPassword(hash, []byte(password))
}

func createAdminToken() (string, error) {
	// Issues a JWT for the admin session cookie.
	claims := &AdminClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "meta-terminal-admin",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	secret, err := adminJWTSecret()
	if err != nil {
		return "", err
	}
	return token.SignedString([]byte(secret))
}

func adminPasswordPath() string {
	// Stores admin credentials alongside other data files.
	dataDir := config.Load().DataDir
	if dataDir == "" {
		dataDir = "data"
	}
	return filepath.Join(dataDir, "admin_password")
}

func adminJWTSecret() (string, error) {
	// Admin auth secret must be provided explicitly.
	cfg := config.Load()
	if cfg.AdminAuthSecret == "" {
		return "", fmt.Errorf("admin auth secret is required")
	}
	return cfg.AdminAuthSecret, nil
}

// adminCookieName returns the configured admin session cookie name.
func adminCookieName() string {
	return config.Load().AdminCookieName
}

// adminCookiePath returns the configured admin session cookie path.
func adminCookiePath() string {
	return config.Load().AdminCookiePath
}

// adminCookieMaxAge returns the configured admin cookie max age.
func adminCookieMaxAge() int {
	return config.Load().AdminCookieMaxAge
}
