package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/maxonlinux/meta-terminal-go/internal/auth"
	"github.com/maxonlinux/meta-terminal-go/internal/impersonation"
	"github.com/maxonlinux/meta-terminal-go/internal/otp"
	"github.com/maxonlinux/meta-terminal-go/internal/users"
	"github.com/maxonlinux/meta-terminal-go/pkg/config"
)

type AuthHandler struct {
	authService     *users.Service
	jwtService      *auth.JWTService
	otpService      *otp.Service
	impService      *impersonation.Service
	jwtCookieName   string
	jwtCookiePath   string
	jwtCookieMaxAge int
}

func NewAuthHandler(authService *users.Service, jwtService *auth.JWTService, otpService *otp.Service, impService *impersonation.Service, cfg config.Config) *AuthHandler {
	return &AuthHandler{
		authService:     authService,
		jwtService:      jwtService,
		otpService:      otpService,
		impService:      impService,
		jwtCookieName:   cfg.JwtCookieName,
		jwtCookiePath:   cfg.JwtCookiePath,
		jwtCookieMaxAge: cfg.JwtCookieMaxAge,
	}
}

// setAuthCookie writes the JWT session cookie to the response.
func (h *AuthHandler) setAuthCookie(c echo.Context, token string) {
	c.SetCookie(&http.Cookie{
		Name:     h.jwtCookieName,
		Value:    token,
		Path:     h.jwtCookiePath,
		MaxAge:   h.jwtCookieMaxAge,
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
	})
}

// clearAuthCookie expires the JWT session cookie.
func (h *AuthHandler) clearAuthCookie(c echo.Context) {
	c.SetCookie(&http.Cookie{
		Name:     h.jwtCookieName,
		Value:    "",
		Path:     h.jwtCookiePath,
		MaxAge:   -1,
		HttpOnly: true,
	})
}

type RegisterRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
}

func (h *AuthHandler) Register(c echo.Context) error {
	var req RegisterRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	userID, err := h.authService.Register(req.Username, req.Password, req.Email, req.Phone)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	_, _ = h.otpService.Generate(req.Username)
	return c.JSON(http.StatusCreated, map[string]interface{}{"userId": uint64(userID)})
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *AuthHandler) Login(c echo.Context) error {
	var req LoginRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	user, err := h.authService.GetUserByUsername(req.Username)
	if err != nil || user == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
	}
	profile, _ := h.authService.GetProfile(user.UserID)
	if profile != nil && !profile.IsActive {
		_, _ = h.otpService.Generate(user.Username)
		return c.JSON(http.StatusForbidden, map[string]string{"error": "USER_NOT_ACTIVE"})
	}

	if !h.authService.ValidatePassword(user, req.Password) {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
	}

	token, err := h.jwtService.CreateToken(user.UserID, user.Username)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create token"})
	}

	h.setAuthCookie(c, token)

	return c.JSON(http.StatusOK, map[string]interface{}{"token": token})
}

type RecoveryRequest struct {
	Username string `json:"username"`
	OTP      string `json:"otp"`
}

type ActivateRequest struct {
	Username string `json:"username"`
	OTP      string `json:"otp"`
}

func (h *AuthHandler) Recovery(c echo.Context) error {
	var req RecoveryRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	user, err := h.authService.GetUserByUsername(req.Username)
	if err != nil || user == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "user not found"})
	}
	if err := h.otpService.Verify(user.Username, req.OTP); err != nil {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "INVALID_OTP"})
	}
	token, err := h.jwtService.CreateToken(user.UserID, user.Username)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create token"})
	}
	h.setAuthCookie(c, token)
	return c.JSON(http.StatusOK, map[string]interface{}{"message": "USER_LOGIN_SUCCESS"})
}

func (h *AuthHandler) Activate(c echo.Context) error {
	var req ActivateRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	user, err := h.authService.GetUserByUsername(req.Username)
	if err != nil || user == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "user not found"})
	}
	profile, _ := h.authService.GetProfile(user.UserID)
	if profile != nil && profile.IsActive {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "USER_ALREADY_ACTIVATED"})
	}
	if err := h.otpService.Verify(user.Username, req.OTP); err != nil {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "INVALID_OTP"})
	}
	_ = h.authService.SetActive(user.UserID, true)
	return c.JSON(http.StatusOK, map[string]string{"message": "USER_ACTIVATE_SUCCESS"})
}

func (h *AuthHandler) Impersonate(c echo.Context) error {
	code := c.Param("code")
	if code == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "code is required"})
	}
	userID, err := h.impService.Redeem(code)
	if err != nil {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "invalid impersonation code"})
	}
	user, err := h.authService.GetUserByID(userID)
	if err != nil || user == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "user not found"})
	}
	token, err := h.jwtService.CreateToken(user.UserID, user.Username)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create token"})
	}
	h.setAuthCookie(c, token)
	return c.JSON(http.StatusOK, map[string]interface{}{"token": token})
}

func (h *AuthHandler) Logout(c echo.Context) error {
	h.clearAuthCookie(c)

	return c.NoContent(http.StatusNoContent)
}
