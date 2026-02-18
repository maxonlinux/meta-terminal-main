package api

import (
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/maxonlinux/meta-terminal-go/internal/otp"
	"github.com/maxonlinux/meta-terminal-go/internal/users"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

type OTPHandler struct {
	otpService  *otp.Service
	userService *users.Service
}

func NewOTPHandler(otpService *otp.Service, userService *users.Service) *OTPHandler {
	return &OTPHandler{otpService: otpService, userService: userService}
}

type OTPRequest struct {
	Username string `json:"username"`
}

type OTPValidateRequest struct {
	Username string `json:"username"`
	OTP      string `json:"otp"`
}

func (h *OTPHandler) Generate(c *echo.Context) error {
	var req OTPRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	user, err := h.userService.GetUserByUsername(req.Username)
	if err != nil || user == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "user not found"})
	}
	email, phone := h.profileContact(user.UserID)
	_, err = h.otpService.Generate(user.Username, email, phone)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to generate otp"})
	}
	return c.JSON(http.StatusCreated, map[string]string{"message": "OTP_SENT"})
}

func (h *OTPHandler) Validate(c *echo.Context) error {
	var req OTPValidateRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	user, err := h.userService.GetUserByUsername(req.Username)
	if err != nil || user == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "user not found"})
	}
	if err := h.otpService.Verify(user.Username, req.OTP); err != nil {
		code := "INVALID_OTP"
		if err == otp.ErrNotGenerated {
			code = "OTP_NOT_GENERATED"
		}
		return c.JSON(http.StatusForbidden, map[string]string{"error": code})
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "OTP_VALID"})
}

func (h *OTPHandler) Check(c *echo.Context) error {
	var req OTPRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	user, err := h.userService.GetUserByUsername(req.Username)
	if err != nil || user == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "user not found"})
	}
	if !h.otpService.Check(user.Username) {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "OTP_REQUIRED"})
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "OTP_VALID"})
}

func (h *OTPHandler) profileContact(userID types.UserID) (string, string) {
	profile, _ := h.userService.GetProfile(userID)
	if profile == nil {
		return "", ""
	}
	return profile.Email, profile.Phone
}
