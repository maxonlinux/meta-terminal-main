package gateway

import (
	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"github.com/maxonlinux/meta-terminal-go/internal/engine"
	"github.com/maxonlinux/meta-terminal-go/internal/gateway/handlers"
	"github.com/maxonlinux/meta-terminal-go/internal/users"
)

type Gateway struct {
	e                *echo.Echo
	engine           *engine.Engine
	authHandler      *handlers.AuthHandler
	ordersHandler    *handlers.OrdersHandler
	positionsHandler *handlers.PositionsHandler
	balancesHandler  *handlers.BalancesHandler
	marketHandler    *handlers.MarketHandler
	jwtService       *users.JWTService
}

func New(eng *engine.Engine, userStore users.UserStore, jwtService *users.JWTService, authService *users.Service) *echo.Echo {
	h := &Gateway{
		engine:           eng,
		authHandler:      handlers.NewAuthHandler(authService, jwtService),
		ordersHandler:    handlers.NewOrdersHandler(eng),
		positionsHandler: handlers.NewPositionsHandler(eng),
		balancesHandler:  handlers.NewBalancesHandler(eng),
		marketHandler:    handlers.NewMarketHandler(eng),
		jwtService:       jwtService,
	}

	e := echo.New()
	e.HideBanner = true

	e.Use(echomw.Recover())
	e.Use(echomw.RequestID())
	e.Use(h.CORSMiddleware())

	e.GET("/health", h.Health)

	api := e.Group("/api/v1")

	authGroup := api.Group("/auth")
	authGroup.POST("/register", h.authHandler.Register)
	authGroup.POST("/login", h.authHandler.Login)
	authGroup.POST("/logout", h.authHandler.Logout)
	authGroup.POST("/recovery", h.authHandler.Recovery)

	authenticated := api.Group("")
	authenticated.Use(h.AuthMiddleware())

	ordersGroup := authenticated.Group("/orders")
	ordersGroup.POST("", h.ordersHandler.CreateOrder)
	ordersGroup.GET("", h.ordersHandler.GetOrders)
	ordersGroup.GET("/:id", h.ordersHandler.GetOrder)
	ordersGroup.DELETE("/:id", h.ordersHandler.CancelOrder)
	ordersGroup.PUT("/:id/amend", h.ordersHandler.AmendOrder)

	positionsGroup := authenticated.Group("/positions")
	positionsGroup.GET("", h.positionsHandler.GetPositions)
	positionsGroup.PUT("/leverage", h.positionsHandler.SetLeverage)

	balancesGroup := authenticated.Group("/balances")
	balancesGroup.GET("", h.balancesHandler.GetBalances)

	marketGroup := api.Group("/market")
	marketGroup.GET("/book", h.marketHandler.GetOrderBook)
	marketGroup.GET("/trades", h.marketHandler.GetTrades)

	instrumentsGroup := api.Group("/instruments")
	instrumentsGroup.GET("", h.marketHandler.GetInstruments)

	return e
}

func (h *Gateway) Health(c echo.Context) error {
	return c.JSON(200, map[string]string{
		"status": "ok",
	})
}

func (h *Gateway) AuthMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			cookie, err := c.Request().Cookie(users.CookieName)
			if err != nil {
				return c.JSON(401, map[string]interface{}{
					"success": false,
					"error": map[string]string{
						"code":    "401",
						"message": "missing authentication",
					},
				})
			}

			claims, err := h.jwtService.ValidateToken(cookie.Value)
			if err != nil {
				msg := "invalid token"
				if err == users.ErrExpiredToken {
					msg = "token expired"
				}
				return c.JSON(401, map[string]interface{}{
					"success": false,
					"error": map[string]string{
						"code":    "401",
						"message": msg,
					},
				})
			}

			c.Set("user", claims)
			return next(c)
		}
	}
}

func (h *Gateway) CORSMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Set("Access-Control-Allow-Origin", "*")
			c.Response().Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Response().Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			c.Response().Header().Set("Access-Control-Allow-Credentials", "true")

			if c.Request().Method == "OPTIONS" {
				return c.NoContent(204)
			}

			return next(c)
		}
	}
}

func Run(eng *engine.Engine, dataDir, addr string) error {
	store, err := users.NewSQLiteStore(dataDir)
	if err != nil {
		return err
	}
	defer store.Close()

	jwtService := users.NewJWTService()
	authService := users.NewService(store)

	e := New(eng, store, jwtService, authService)
	return e.Start(addr)
}
