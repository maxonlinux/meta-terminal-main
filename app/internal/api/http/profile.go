package api

import (
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/maxonlinux/meta-terminal-go/internal/users"
)

type ProfileHandler struct {
	service *users.Service
}

type ProfileResponse struct {
	ID       int64   `json:"id"`
	Email    string  `json:"email"`
	Username string  `json:"username"`
	Phone    string  `json:"phone"`
	Name     *string `json:"name"`
	Surname  *string `json:"surname"`
	IsActive bool    `json:"isActive"`
}

func NewProfileHandler(service *users.Service) *ProfileHandler {
	return &ProfileHandler{service: service}
}

func (h *ProfileHandler) Get(c *echo.Context) error {
	claims := getUser(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "authentication required"})
	}

	profile, err := h.service.GetProfile(claims.UserID)
	if err != nil || profile == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "user not found"})
	}

	return c.JSON(http.StatusOK, ProfileResponse{
		ID:       profile.UserID,
		Email:    profile.Email,
		Username: profile.Username,
		Phone:    profile.Phone,
		Name:     profile.Name,
		Surname:  profile.Surname,
		IsActive: profile.IsActive,
	})
}
