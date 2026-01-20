package gateway

import (
	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"github.com/maxonlinux/meta-terminal-go/internal/engine"
	"github.com/maxonlinux/meta-terminal-go/internal/gateway/auth"
	"github.com/maxonlinux/meta-terminal-go/pkg/persistence"
)

func New(eng *engine.Engine, userStore auth.UserStore, jwtService *auth.JWTService, authService *auth.Service) *echo.Echo {
	h := NewHandler(eng, jwtService, authService)

	e := echo.New()
	e.HideBanner = true

	e.Use(echomw.Recover())
	e.Use(echomw.RequestID())
	e.Use(CORSMiddleware())

	e.GET("/health", h.Health)

	api := e.Group("/api/v1")

	authGroup := api.Group("/auth")
	authGroup.POST("/register", h.Register)
	authGroup.POST("/login", h.Login)
	authGroup.POST("/logout", h.Logout)
	authGroup.POST("/recovery", h.Recovery)

	authenticated := api.Group("")
	authenticated.Use(AuthMiddleware(jwtService))

	ordersGroup := authenticated.Group("/orders")
	ordersGroup.POST("", h.CreateOrder)
	ordersGroup.GET("", h.GetOrders)
	ordersGroup.GET("/:id", h.GetOrder)
	ordersGroup.DELETE("/:id", h.CancelOrder)
	ordersGroup.PUT("/:id/amend", h.AmendOrder)

	positionsGroup := authenticated.Group("/positions")
	positionsGroup.GET("", h.GetPositions)
	positionsGroup.PUT("/leverage", h.SetLeverage)

	balancesGroup := authenticated.Group("/balances")
	balancesGroup.GET("", h.GetBalances)

	marketGroup := api.Group("/market")
	marketGroup.GET("/book", h.GetOrderBook)
	marketGroup.GET("/trades", h.GetTrades)

	instrumentsGroup := api.Group("/instruments")
	instrumentsGroup.GET("", h.GetInstruments)

	return e
}

func Run(eng *engine.Engine, pkv *persistence.PebbleKV, addr string) error {
	userStore := auth.NewUserStore(pkv)
	jwtService := auth.NewJWTService()
	authService := auth.NewService(userStore)

	e := New(eng, userStore, jwtService, authService)
	return e.Start(addr)
}
