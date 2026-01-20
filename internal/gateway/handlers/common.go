package handlers

import (
	"github.com/labstack/echo/v4"
)

type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *APIError   `json:"error,omitempty"`
}

type APIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func Success(c echo.Context, data interface{}) error {
	return c.JSON(200, APIResponse{
		Success: true,
		Data:    data,
	})
}

func Created(c echo.Context, data interface{}) error {
	return c.JSON(201, APIResponse{
		Success: true,
		Data:    data,
	})
}

func BadRequest(c echo.Context, message string) error {
	return c.JSON(400, APIResponse{
		Success: false,
		Error: &APIError{
			Code:    400,
			Message: message,
		},
	})
}

func Unauthorized(c echo.Context, message string) error {
	return c.JSON(401, APIResponse{
		Success: false,
		Error: &APIError{
			Code:    401,
			Message: message,
		},
	})
}

func NotFound(c echo.Context, message string) error {
	return c.JSON(404, APIResponse{
		Success: false,
		Error: &APIError{
			Code:    404,
			Message: message,
		},
	})
}

func InternalError(c echo.Context, message string) error {
	return c.JSON(500, APIResponse{
		Success: false,
		Error: &APIError{
			Code:    500,
			Message: message,
		},
	})
}
