package gateway

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/maxonlinux/meta-terminal-go/internal/gateway/auth"
)

type ContextKey string

const UserContextKey ContextKey = "user"

func AuthMiddleware(jwtService *auth.JWTService) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			cookie, err := c.Request().Cookie(auth.CookieName)
			if err != nil {
				return Unauthorized(c, "missing authentication")
			}

			claims, err := jwtService.ValidateToken(cookie.Value)
			if err != nil {
				if err == auth.ErrExpiredToken {
					return Unauthorized(c, "token expired")
				}
				return Unauthorized(c, "invalid token")
			}

			c.Set(string(UserContextKey), claims)
			return next(c)
		}
	}
}

func GetUserFromContext(c echo.Context) *auth.Claims {
	if v := c.Get(string(UserContextKey)); v != nil {
		if claims, ok := v.(*auth.Claims); ok {
			return claims
		}
	}
	return nil
}

func GetUserIDFromContext(c echo.Context) (uint64, bool) {
	claims := GetUserFromContext(c)
	if claims == nil {
		return 0, false
	}
	return uint64(claims.UserID), true
}

func CORSMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Set("Access-Control-Allow-Origin", "*")
			c.Response().Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Response().Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			c.Response().Header().Set("Access-Control-Allow-Credentials", "true")

			if c.Request().Method == http.MethodOptions {
				return c.NoContent(http.StatusNoContent)
			}

			return next(c)
		}
	}
}

func RequireAuth(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if GetUserFromContext(c) == nil {
			return Unauthorized(c, "authentication required")
		}
		return next(c)
	}
}

func ParseBoolQueryParam(c echo.Context, name string) bool {
	return strings.ToLower(c.QueryParam(name)) == "true" ||
		c.QueryParam(name) == "1"
}
