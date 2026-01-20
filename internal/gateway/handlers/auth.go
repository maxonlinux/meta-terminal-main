package handlers

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
		return BadRequest(c, "invalid request body")
	}

	userID, err := h.authService.Register(req.Username, req.Password)
	if err != nil {
		return BadRequest(c, err.Error())
	}

	return Success(c, map[string]interface{}{
		"userId": uint64(userID),
	})
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *AuthHandler) Login(c echo.Context) error {
	var req LoginRequest
	if err := c.Bind(&req); err != nil {
		return BadRequest(c, "invalid request body")
	}

	user, err := h.authService.GetUserByUsername(req.Username)
	if err != nil || user == nil {
		return Unauthorized(c, "invalid credentials")
	}

	if !h.authService.ValidatePassword(user, req.Password) {
		return Unauthorized(c, "invalid credentials")
	}

	token, err := h.jwtService.CreateToken(user.UserID, user.Username)
	if err != nil {
		return InternalError(c, "failed to create token")
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

	return Success(c, map[string]interface{}{
		"token": token,
	})
}

func (h *AuthHandler) Logout(c echo.Context) error {
	c.SetCookie(&http.Cookie{
		Name:     users.CookieName,
		Value:    "",
		Path:     users.CookiePath,
		MaxAge:   -1,
		HttpOnly: true,
	})

	return Success(c, nil)
}

func (h *AuthHandler) Recovery(c echo.Context) error {
	return Success(c, map[string]string{
		"message": "recovery email sent",
	})
}
