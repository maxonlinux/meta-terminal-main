package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/maxonlinux/meta-terminal-go/internal/users"
)

type ProfileHandler struct {
	service *users.Service
}

func NewProfileHandler(service *users.Service) *ProfileHandler {
	return &ProfileHandler{service: service}
}

func (h *ProfileHandler) Get(c echo.Context) error {
	claims := getUser(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "authentication required"})
	}

	profile, err := h.service.GetProfile(claims.UserID)
	if err != nil || profile == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "user not found"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"id":       uint64(profile.UserID),
		"email":    profile.Email,
		"username": profile.Username,
		"phone":    profile.Phone,
		"name":     profile.Name,
		"surname":  profile.Surname,
		"isActive": profile.IsActive,
	})
}
