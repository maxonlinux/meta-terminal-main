package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v5"
	"github.com/maxonlinux/meta-terminal-go/pkg/logging"
)

// UserActionLogger records mutating user actions with structured fields.
func (r *Router) UserActionLogger() echo.MiddlewareFunc {
	return actionLogger("user")
}

// AdminActionLogger records mutating admin actions with structured fields.
func (r *Router) AdminActionLogger() echo.MiddlewareFunc {
	return actionLogger("admin")
}

// PublicActionLogger records mutating public actions (register/login/etc.).
func (r *Router) PublicActionLogger() echo.MiddlewareFunc {
	return actionLogger("public")
}

func actionLogger(scope string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			err := next(c)
			method := c.Request().Method
			if !isMutatingMethod(method) {
				return err
			}
			actorType := scope
			var actorID string
			if claims := getUser(c); claims != nil {
				actorType = "user"
				actorID = strconv.FormatInt(claims.UserID, 10)
			} else if getAdmin(c) != nil {
				actorType = "admin"
			}
			requestID := c.Response().Header().Get(echo.HeaderXRequestID)
			if requestID == "" {
				requestID = c.Request().Header.Get(echo.HeaderXRequestID)
			}
			status := 0
			if resp, ok := c.Response().(*echo.Response); ok {
				status = resp.Status
			}
			logging.Log().Info().
				Str("event", "user_action").
				Str("actor_type", actorType).
				Str("actor_id", actorID).
				Str("method", method).
				Str("path", c.Path()).
				Int("status", status).
				Str("request_id", requestID).
				Str("ip", c.RealIP()).
				Str("user_agent", c.Request().UserAgent()).
				Msg("user action")
			return err
		}
	}
}

func isMutatingMethod(method string) bool {
	return strings.EqualFold(method, http.MethodPost) ||
		strings.EqualFold(method, http.MethodPut) ||
		strings.EqualFold(method, http.MethodPatch) ||
		strings.EqualFold(method, http.MethodDelete)
}

func getAdmin(c *echo.Context) *AdminClaims {
	if v := c.Get("admin"); v != nil {
		if claims, ok := v.(*AdminClaims); ok {
			return claims
		}
	}
	return nil
}
