package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"github.com/maxonlinux/meta-terminal-go/internal/api/ws"
	"github.com/maxonlinux/meta-terminal-go/internal/engine"
	"github.com/maxonlinux/meta-terminal-go/internal/impersonation"
	"github.com/maxonlinux/meta-terminal-go/internal/otp"
	"github.com/maxonlinux/meta-terminal-go/internal/persistence"
	"github.com/maxonlinux/meta-terminal-go/internal/query"
	"github.com/maxonlinux/meta-terminal-go/internal/users"
)

type Router struct {
	AuthHandler      *AuthHandler
	OtpHandler       *OTPHandler
	UserHandler      *UserHandler
	OrdersHandler    *OrdersHandler
	PositionsHandler *PositionsHandler
	BalancesHandler  *BalancesHandler
	MarketHandler    *MarketHandler
	ProfileHandler   *ProfileHandler
	HistoryHandler   *HistoryHandler
	WsHandler        *ws.WsHandler
	jwtService       *users.JWTService
	otpService       *otp.Service
	userService      *users.Service
}

func NewRouter(eng *engine.Engine, queryService *query.Service, persistenceStore *persistence.Store, userStore users.UserStore, jwtService *users.JWTService, authService *users.Service, otpService *otp.Service, impService *impersonation.Service) *Router {
	return &Router{
		AuthHandler:      NewAuthHandler(authService, jwtService, otpService, impService),
		OtpHandler:       NewOTPHandler(otpService, authService),
		UserHandler:      NewUserHandler(authService, queryService, eng, persistenceStore),
		OrdersHandler:    NewOrdersHandler(eng, queryService),
		PositionsHandler: NewPositionsHandler(eng, queryService),
		BalancesHandler:  NewBalancesHandler(queryService),
		MarketHandler:    NewMarketHandler(queryService),
		ProfileHandler:   NewProfileHandler(authService),
		HistoryHandler:   NewHistoryHandler(persistenceStore),
		jwtService:       jwtService,
		otpService:       otpService,
		userService:      authService,
	}
}

func (r *Router) SetWsHandler(handler *ws.WsHandler) {
	r.WsHandler = handler
}

func (r *Router) Register(e *echo.Echo) {
	e.Use(echomw.Recover())
	e.Use(echomw.RequestID())
	e.Use(r.CORSMiddleware())

	e.GET("/health", r.Health)

	if r.WsHandler != nil {
		wsGroup := e.Group("/ws")
		wsGroup.GET("/market", r.WsHandler.Market)
		wsGroup.GET("/events", r.WsHandler.Events)
	}

	api := e.Group("/api/v1")

	authGroup := api.Group("/auth")
	authGroup.POST("/register", r.AuthHandler.Register)
	authGroup.POST("/login", r.AuthHandler.Login)
	authGroup.POST("/logout", r.AuthHandler.Logout)
	authGroup.POST("/recovery", r.AuthHandler.Recovery)
	authGroup.POST("/activate", r.AuthHandler.Activate)
	authGroup.GET("/impersonate/:code", r.AuthHandler.Impersonate)

	otpGroup := api.Group("/otp")
	otpGroup.POST("/generate", r.OtpHandler.Generate)
	otpGroup.POST("/validate", r.OtpHandler.Validate)
	otpGroup.POST("/check", r.OtpHandler.Check)

	authenticated := api.Group("")
	authenticated.Use(r.AuthMiddleware())
	otpRequired := authenticated.Group("")
	otpRequired.Use(r.OTPMiddleware())

	ordersGroup := authenticated.Group("/orders")
	ordersGroup.POST("", r.OrdersHandler.Create)
	ordersGroup.GET("", r.OrdersHandler.List)
	ordersGroup.GET("/:id", r.OrdersHandler.Get)
	ordersGroup.DELETE("/:id", r.OrdersHandler.Cancel)
	ordersGroup.PUT("/:id/amend", r.OrdersHandler.Amend)

	positionsGroup := authenticated.Group("/positions")
	positionsGroup.GET("", r.PositionsHandler.List)
	positionsGroup.PUT("/leverage", r.PositionsHandler.SetLeverage)
	positionsGroup.PATCH("", r.PositionsHandler.UpdateTpSl)

	balancesGroup := authenticated.Group("/balances")
	balancesGroup.GET("", r.BalancesHandler.List)

	profileGroup := authenticated.Group("/profile")
	profileGroup.GET("", r.ProfileHandler.Get)

	userGroup := authenticated.Group("/user")
	userGroup.GET("", r.UserHandler.Profile)
	userGroup.PATCH("", r.UserHandler.UpdateProfile)
	userGroup.GET("/plan", r.UserHandler.Plan)
	userGroup.GET("/balances", r.UserHandler.Balances)
	userGroup.GET("/balance", r.UserHandler.Balance)

	settingsGroup := authenticated.Group("/user/settings")
	settingsGroup.GET("", r.UserHandler.Settings)
	settingsGroup.GET("/address", r.UserHandler.Address)

	settingsOtpGroup := otpRequired.Group("/user/settings")
	settingsOtpGroup.PATCH("", r.UserHandler.UpdateSettings)
	settingsOtpGroup.PATCH("/address", r.UserHandler.UpdateAddress)
	settingsOtpGroup.PUT("/password", r.UserHandler.UpdatePassword)

	fundingGroup := authenticated.Group("/user/funding")
	fundingGroup.GET("", r.UserHandler.FundingList)

	fundingOtpGroup := otpRequired.Group("/user/funding")
	fundingOtpGroup.POST("/deposit", r.UserHandler.FundingDeposit)
	fundingOtpGroup.POST("/withdraw", r.UserHandler.FundingWithdraw)

	historyGroup := authenticated.Group("/history")
	historyGroup.GET("/orders", r.HistoryHandler.Orders)
	historyGroup.GET("/fills", r.HistoryHandler.Fills)

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

func (r *Router) OTPMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			claims := getUser(c)
			if claims == nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "authentication required"})
			}
			user, err := r.userService.GetUserByID(claims.UserID)
			if err != nil || user == nil {
				return c.JSON(http.StatusNotFound, map[string]string{"error": "user not found"})
			}
			if !r.otpService.Check(user.Username) {
				return c.JSON(http.StatusPreconditionRequired, map[string]string{"error": "OTP_REQUIRED"})
			}
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
