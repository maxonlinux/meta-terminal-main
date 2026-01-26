package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	apihandlers "github.com/maxonlinux/meta-terminal-go/internal/api/http"
	"github.com/maxonlinux/meta-terminal-go/internal/engine"
	"github.com/maxonlinux/meta-terminal-go/internal/query"
	"github.com/maxonlinux/meta-terminal-go/internal/registry"
	"github.com/maxonlinux/meta-terminal-go/internal/users"
	"github.com/maxonlinux/meta-terminal-go/pkg/config"
	"github.com/maxonlinux/meta-terminal-go/pkg/outbox"
	"github.com/maxonlinux/meta-terminal-go/pkg/persistence"
)

func main() {
	cfg := config.Load()

	reg := registry.New()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	loader := registry.NewLoader(cfg, reg)
	go loader.Start(ctx)

	tradingPath := filepath.Join(cfg.DataDir, "trading")
	store, err := persistence.Open(tradingPath)
	if err != nil {
		log.Fatalf("persistence open: %v", err)
	}
	defer store.Close()

	ob, err := outbox.Open(cfg.DataDir, store.DB())
	if err != nil {
		log.Fatalf("outbox open: %v", err)
	}
	defer ob.Close()
	ob.Start()

	eng := engine.NewEngine(store, ob, reg, nil)

	go func() {
		if err := runServer(eng, cfg); err != nil {
			log.Printf("gateway error: %v", err)
		}
	}()

	<-ctx.Done()
	eng.Shutdown()
}

func runServer(eng *engine.Engine, cfg config.Config) error {
	userStore, err := users.NewSQLiteStore(cfg.DataDir)
	if err != nil {
		return err
	}
	defer userStore.Close()

	jwtService := users.NewJWTService()
	authService := users.NewService(userStore)

	queryService := query.New(eng.Registry(), eng.Portfolio(), eng.Store(), eng.TradeFeed(), eng.ReadBook)
	router := apihandlers.NewRouter(eng, queryService, userStore, jwtService, authService)

	e := echo.New()
	e.HideBanner = true

	e.Use(echomw.Recover())
	e.Use(echomw.RequestID())
	e.Use(corsMiddleware())

	e.GET("/health", healthHandler)

	api := e.Group("/api/v1")

	authGroup := api.Group("/auth")
	authGroup.POST("/register", router.AuthHandler.Register)
	authGroup.POST("/login", router.AuthHandler.Login)
	authGroup.POST("/logout", router.AuthHandler.Logout)

	authenticated := api.Group("")
	authenticated.Use(authMiddleware(jwtService))

	ordersGroup := authenticated.Group("/orders")
	ordersGroup.POST("", router.OrdersHandler.Create)
	ordersGroup.GET("", router.OrdersHandler.List)
	ordersGroup.GET("/:id", router.OrdersHandler.Get)
	ordersGroup.DELETE("/:id", router.OrdersHandler.Cancel)
	ordersGroup.PUT("/:id/amend", router.OrdersHandler.Amend)

	positionsGroup := authenticated.Group("/positions")
	positionsGroup.GET("", router.PositionsHandler.List)
	positionsGroup.PUT("/leverage", router.PositionsHandler.SetLeverage)

	balancesGroup := authenticated.Group("/balances")
	balancesGroup.GET("", router.BalancesHandler.List)

	marketGroup := api.Group("/market")
	marketGroup.GET("/book", router.MarketHandler.OrderBook)
	marketGroup.GET("/trades", router.MarketHandler.Trades)

	instrumentsGroup := api.Group("/instruments")
	instrumentsGroup.GET("", router.MarketHandler.Instruments)

	return e.Start(":8080")
}

func healthHandler(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func authMiddleware(jwtService *users.JWTService) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			cookie, err := c.Request().Cookie(users.CookieName)
			if err != nil {
				return c.JSON(http.StatusUnauthorized, map[string]interface{}{
					"success": false,
					"error":   map[string]string{"code": "401", "message": "missing authentication"},
				})
			}

			claims, err := jwtService.ValidateToken(cookie.Value)
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

func corsMiddleware() echo.MiddlewareFunc {
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
