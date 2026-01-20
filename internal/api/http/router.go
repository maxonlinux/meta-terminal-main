package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"github.com/maxonlinux/meta-terminal-go/internal/engine"
	"github.com/maxonlinux/meta-terminal-go/internal/users"
)

type Router struct {
	AuthHandler      *AuthHandler
	OrdersHandler    *OrdersHandler
	PositionsHandler *PositionsHandler
	BalancesHandler  *BalancesHandler
	MarketHandler    *MarketHandler
	jwtService       *users.JWTService
}

func NewRouter(eng *engine.Engine, userStore users.UserStore, jwtService *users.JWTService, authService *users.Service) *Router {
	return &Router{
		AuthHandler:      NewAuthHandler(authService, jwtService),
		OrdersHandler:    NewOrdersHandler(eng),
		PositionsHandler: NewPositionsHandler(eng),
		BalancesHandler:  NewBalancesHandler(eng),
		MarketHandler:    NewMarketHandler(eng),
		jwtService:       jwtService,
	}
}

func (r *Router) Register(e *echo.Echo) {
	e.Use(echomw.Recover())
	e.Use(echomw.RequestID())
	e.Use(r.CORSMiddleware())

	e.GET("/health", r.Health)

	api := e.Group("/api/v1")

	authGroup := api.Group("/auth")
	authGroup.POST("/register", r.AuthHandler.Register)
	authGroup.POST("/login", r.AuthHandler.Login)
	authGroup.POST("/logout", r.AuthHandler.Logout)

	authenticated := api.Group("")
	authenticated.Use(r.AuthMiddleware())

	ordersGroup := authenticated.Group("/orders")
	ordersGroup.POST("", r.OrdersHandler.Create)
	ordersGroup.GET("", r.OrdersHandler.List)
	ordersGroup.GET("/:id", r.OrdersHandler.Get)
	ordersGroup.DELETE("/:id", r.OrdersHandler.Cancel)
	ordersGroup.PUT("/:id/amend", r.OrdersHandler.Amend)

	positionsGroup := authenticated.Group("/positions")
	positionsGroup.GET("", r.PositionsHandler.List)
	positionsGroup.PUT("/leverage", r.PositionsHandler.SetLeverage)

	balancesGroup := authenticated.Group("/balances")
	balancesGroup.GET("", r.BalancesHandler.List)

	marketGroup := api.Group("/market")
	marketGroup.GET("/book", r.MarketHandler.OrderBook)
	marketGroup.GET("/trades", r.MarketHandler.Trades)

	instrumentsGroup := api.Group("/instruments")
	instrumentsGroup.GET("", r.MarketHandler.Instruments)
}

func (r *Router) Health(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func (r *Router) AuthMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			cookie, err := c.Request().Cookie(users.CookieName)
			if err != nil {
				return c.JSON(http.StatusUnauthorized, map[string]interface{}{
					"success": false,
					"error":   map[string]string{"code": "401", "message": "missing authentication"},
				})
			}

			claims, err := r.jwtService.ValidateToken(cookie.Value)
			if err != nil {
				msg := "invalid token"
				if err == users.ErrExpiredToken {
					msg = "token expired"
				}
				return c.JSON(http.StatusUnauthorized, map[string]interface{}{
					"success": false,
					"error":   map[string]string{"code": "401", "message": msg},
				})
			}

			c.Set("user", claims)
			return next(c)
		}
	}
}

func (r *Router) CORSMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Set("Access-Control-Allow-Origin", "*")
			c.Response().Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Response().Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			c.Response().Header().Set("Access-Control-Allow-Credentials", "true")

			if c.Request().Method == "OPTIONS" {
				return c.NoContent(http.StatusNoContent)
			}

			return next(c)
		}
	}
}
