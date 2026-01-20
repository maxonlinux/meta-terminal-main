package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/maxonlinux/meta-terminal-go/internal/users"
)

type AuthHandler struct {
	authService *users.Service
	jwtService  *users.JWTService
}

func NewAuthHandler(authService *users.Service, jwtService *users.JWTService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		jwtService:  jwtService,
	}
}

type RegisterRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *AuthHandler) Register(c echo.Context) error {
	var req RegisterRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	userID, err := h.authService.Register(req.Username, req.Password)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

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

	if !h.authService.ValidatePassword(user, req.Password) {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
	}

	token, err := h.jwtService.CreateToken(user.UserID, user.Username)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create token"})
	}

	c.SetCookie(&http.Cookie{
		Name:     users.CookieName,
		Value:    token,
		Path:     users.CookiePath,
		MaxAge:   users.CookieMaxAge,
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
	})

	return c.JSON(http.StatusOK, map[string]interface{}{"token": token})
}

func (h *AuthHandler) Logout(c echo.Context) error {
	c.SetCookie(&http.Cookie{
		Name:     users.CookieName,
		Value:    "",
		Path:     users.CookiePath,
		MaxAge:   -1,
		HttpOnly: true,
	})

	return c.NoContent(http.StatusNoContent)
}
