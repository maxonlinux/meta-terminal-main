package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"
	echomw "github.com/labstack/echo/v5/middleware"
	"github.com/maxonlinux/meta-terminal-go/internal/api/ws"
	"github.com/maxonlinux/meta-terminal-go/internal/auth"
	"github.com/maxonlinux/meta-terminal-go/internal/engine"
	"github.com/maxonlinux/meta-terminal-go/internal/impersonation"
	"github.com/maxonlinux/meta-terminal-go/internal/kyc"
	"github.com/maxonlinux/meta-terminal-go/internal/otp"
	"github.com/maxonlinux/meta-terminal-go/internal/persistence"
	"github.com/maxonlinux/meta-terminal-go/internal/plan"
	"github.com/maxonlinux/meta-terminal-go/internal/users"
	"github.com/maxonlinux/meta-terminal-go/internal/wallets"
	"github.com/maxonlinux/meta-terminal-go/pkg/config"
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
	AdminHandler     *AdminHandler
	AdminAuthHandler *AdminAuthHandler
	KYCHandler       *KYCHandler
	WsHandler        *ws.WsHandler
	jwtService       *auth.JWTService
	otpService       *otp.Service
	userService      *users.Service
	otpDisabled      bool
	// jwtCookieName is used to fetch session cookies in middleware.
	jwtCookieName string
}

func NewRouter(eng *engine.Engine, persistenceStore *persistence.Store, jwtService *auth.JWTService, authService *users.Service, otpService *otp.Service, impService *impersonation.Service, planService *plan.Service, planRepo *plan.Repository, walletService *wallets.Service, kycRepo *kyc.Repository, wsHandler *ws.WsHandler, cfg config.Config) (*Router, error) {
	if err := validateRouterDeps(eng, persistenceStore, jwtService, authService, otpService, impService, planService, planRepo, walletService, kycRepo, wsHandler); err != nil {
		return nil, err
	}
	return &Router{
		AuthHandler:      NewAuthHandler(authService, walletService, jwtService, otpService, impService, cfg),
		OtpHandler:       NewOTPHandler(otpService, authService),
		UserHandler:      NewUserHandler(authService, eng, persistenceStore, planService, walletService),
		OrdersHandler:    NewOrdersHandler(eng),
		PositionsHandler: NewPositionsHandler(eng),
		BalancesHandler:  NewBalancesHandler(eng),
		MarketHandler:    NewMarketHandler(eng),
		ProfileHandler:   NewProfileHandler(authService),
		HistoryHandler:   NewHistoryHandler(persistenceStore),
		AdminHandler:     NewAdminHandler(planService, planRepo, walletService, authService, persistenceStore, kycRepo, eng, impService),
		AdminAuthHandler: &AdminAuthHandler{},
		KYCHandler:       NewKYCHandler(kycRepo, authService),
		WsHandler:        wsHandler,
		jwtService:       jwtService,
		otpService:       otpService,
		userService:      authService,
		otpDisabled:      cfg.OtpDisabled,
		jwtCookieName:    cfg.JwtCookieName,
	}, nil
}

func validateRouterDeps(eng *engine.Engine, persistenceStore *persistence.Store, jwtService *auth.JWTService, authService *users.Service, otpService *otp.Service, impService *impersonation.Service, planService *plan.Service, planRepo *plan.Repository, walletService *wallets.Service, kycRepo *kyc.Repository, wsHandler *ws.WsHandler) error {
	missing := make([]string, 0, 8)
	if eng == nil {
		missing = append(missing, "engine")
	}
	if persistenceStore == nil {
		missing = append(missing, "persistence store")
	}
	if jwtService == nil {
		missing = append(missing, "jwt service")
	}
	if authService == nil {
		missing = append(missing, "user service")
	}
	if otpService == nil {
		missing = append(missing, "otp service")
	}
	if impService == nil {
		missing = append(missing, "impersonation service")
	}
	if planService == nil {
		missing = append(missing, "plan service")
	}
	if planRepo == nil {
		missing = append(missing, "plan repository")
	}
	if walletService == nil {
		missing = append(missing, "wallet service")
	}
	if kycRepo == nil {
		missing = append(missing, "kyc repository")
	}
	if wsHandler == nil {
		missing = append(missing, "ws handler")
	}
	if len(missing) > 0 {
		return fmt.Errorf("router dependencies missing: %s", strings.Join(missing, ", "))
	}
	return nil
}

func (r *Router) Register(e *echo.Echo) {
	e.Use(echomw.Recover())
	e.Use(echomw.RequestID())
	// Use RequestLogger to avoid deprecated Logger middleware.
	e.Use(echomw.RequestLogger())
	e.Use(r.CORSMiddleware())

	e.GET("/health", r.Health)

	wsGroup := e.Group("/ws")
	wsGroup.GET("/market", r.WsHandler.Market)
	wsGroup.GET("/events", r.WsHandler.Events)

	api := e.Group("/api/v1")

	authGroup := api.Group("/auth")
	authGroup.Use(r.PublicActionLogger())
	authGroup.POST("/register", r.AuthHandler.Register)
	authGroup.POST("/login", r.AuthHandler.Login)
	authGroup.POST("/logout", r.AuthHandler.Logout)
	authGroup.POST("/recovery", r.AuthHandler.Recovery)
	authGroup.POST("/activate", r.AuthHandler.Activate)
	authGroup.GET("/impersonate/:code", r.AuthHandler.Impersonate)

	otpGroup := api.Group("/otp")
	otpGroup.Use(r.PublicActionLogger())
	otpGroup.POST("/generate", r.OtpHandler.Generate)
	otpGroup.POST("/validate", r.OtpHandler.Validate)
	otpGroup.POST("/check", r.OtpHandler.Check)

	authenticated := api.Group("")
	authenticated.Use(r.AuthMiddleware())
	authenticated.Use(r.UserActionLogger())
	otpRequired := authenticated.Group("")
	otpRequired.Use(r.OTPMiddleware())

	ordersGroup := authenticated.Group("/user/orders")
	ordersGroup.POST("", r.OrdersHandler.Create)
	ordersGroup.GET("", r.OrdersHandler.List)
	ordersGroup.GET("/:id", r.OrdersHandler.Get)
	ordersGroup.DELETE("/:id", r.OrdersHandler.Cancel)
	ordersGroup.PUT("/:id/amend", r.OrdersHandler.Amend)

	positionsGroup := authenticated.Group("/user/positions")
	positionsGroup.GET("", r.PositionsHandler.List)
	positionsGroup.PUT("/leverage", r.PositionsHandler.SetLeverage)
	positionsGroup.PATCH("", r.PositionsHandler.UpdateTpSl)

	balancesGroup := authenticated.Group("/user/balances")
	balancesGroup.GET("", r.BalancesHandler.List)

	profileGroup := authenticated.Group("/user/profile")
	profileGroup.GET("", r.ProfileHandler.Get)
	profileGroup.PATCH("", r.UserHandler.UpdateProfile)

	userGroup := authenticated.Group("/user")
	userGroup.GET("/plan", r.UserHandler.Plan)
	userGroup.GET("/balance", r.UserHandler.Balance)
	userGroup.GET("/wallets", r.UserHandler.Wallets)

	settingsGroup := authenticated.Group("/user/settings")
	settingsGroup.GET("", r.UserHandler.Settings)
	settingsGroup.GET("/address", r.UserHandler.Address)

	settingsOtpGroup := otpRequired.Group("/user/settings")
	settingsOtpGroup.PATCH("", r.UserHandler.UpdateSettings)
	settingsOtpGroup.PATCH("/address", r.UserHandler.UpdateAddress)
	settingsOtpGroup.PUT("/password", r.UserHandler.UpdatePassword)

	kycGroup := authenticated.Group("/user/kyc")
	kycGroup.GET("", r.KYCHandler.GetUserKYC)
	kycGroup.POST("", r.KYCHandler.SubmitKYC)

	fundingGroup := authenticated.Group("/user/funding")
	fundingGroup.GET("", r.UserHandler.FundingList)

	fundingOtpGroup := otpRequired.Group("/user/funding")
	fundingOtpGroup.POST("/deposit", r.UserHandler.FundingDeposit)
	fundingOtpGroup.POST("/withdraw", r.UserHandler.FundingWithdraw)

	historyGroup := authenticated.Group("/user/history")
	historyGroup.GET("/orders", r.HistoryHandler.Orders)
	historyGroup.GET("/fills", r.HistoryHandler.Fills)
	historyGroup.GET("/pnl", r.HistoryHandler.PnL)

	marketGroup := api.Group("/market")
	marketGroup.GET("/book", r.MarketHandler.OrderBook)
	marketGroup.GET("/trades", r.MarketHandler.Trades)

	instrumentsGroup := api.Group("/instruments")
	instrumentsGroup.GET("", r.MarketHandler.Instruments)

	// Backoffice routes and auth endpoints are isolated from user auth.
	adminGroup := api.Group("/admin")

	// Backoffice auth endpoints are public to allow initial setup/login.
	adminAuthGroup := adminGroup.Group("/auth")
	adminAuthGroup.Use(r.PublicActionLogger())
	adminAuthGroup.GET("/status", r.AdminAuthHandler.Status)
	adminAuthGroup.POST("/setup", r.AdminAuthHandler.Setup)
	adminAuthGroup.POST("/login", r.AdminAuthHandler.Login)
	adminAuthGroup.POST("/logout", r.AdminAuthHandler.Logout)

	adminGroup.Use(r.AdminMiddleware())
	adminGroup.Use(r.AdminActionLogger())
	adminGroup.GET("/pending-count", r.AdminHandler.PendingCount)
	adminGroup.GET("/kyc", r.KYCHandler.ListRequests)
	adminGroup.GET("/kyc/:id", r.KYCHandler.GetRequest)
	adminGroup.GET("/kyc/:id/files/:fileId", r.KYCHandler.GetFile)
	adminGroup.PATCH("/kyc/:id", r.KYCHandler.UpdateRequest)
	adminGroup.GET("/users", r.AdminHandler.Users)
	adminGroup.GET("/users/:id", r.AdminHandler.User)
	adminGroup.PATCH("/users/:id/active", r.AdminHandler.SetUserActive)
	adminGroup.GET("/users/:id/address", r.AdminHandler.UserAddress)
	adminGroup.PATCH("/users/:id/address", r.AdminHandler.UpdateUserAddress)
	adminGroup.GET("/users/:id/transactions", r.AdminHandler.UserTransactions)
	adminGroup.GET("/users/:id/impersonate", r.AdminHandler.Impersonate)
	adminGroup.GET("/funding", r.AdminHandler.Funding)
	adminGroup.PATCH("/funding/:id/approve", r.AdminHandler.ApproveFunding)
	adminGroup.PATCH("/funding/:id/cancel", r.AdminHandler.CancelFunding)
	adminGroup.GET("/existing-plans", r.AdminHandler.ExistingPlans)
	adminGroup.GET("/users/:id/plan", r.AdminHandler.GetUserPlan)
	adminGroup.PATCH("/users/:id/plan", r.AdminHandler.SetUserPlan)
	adminGroup.PATCH("/users/:id/reset-plan", r.AdminHandler.ResetUserPlan)
	adminGroup.GET("/wallets", r.AdminHandler.ListWallets)
	adminGroup.POST("/wallets", r.AdminHandler.CreateWallet)
	adminGroup.PATCH("/wallets/:id", r.AdminHandler.UpdateWallet)
	adminGroup.GET("/users/:id/wallets", r.AdminHandler.ListUserWallets)
	adminGroup.PATCH("/users/:id/wallets", r.AdminHandler.AssignUserWallet)
}

func (r *Router) Health(c *echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func (r *Router) AuthMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			cookie, err := c.Request().Cookie(r.jwtCookieName)
			if err != nil {
				return c.JSON(http.StatusUnauthorized, map[string]interface{}{
					"success": false,
					"error":   map[string]string{"code": "401", "message": "missing authentication"},
				})
			}

			claims, err := r.jwtService.ValidateToken(cookie.Value)
			if err != nil {
				msg := "invalid token"
				if err == auth.ErrExpiredToken {
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
		return func(c *echo.Context) error {
			// Allow OTP enforcement to be disabled in dev/test environments.
			if r.otpDisabled {
				return next(c)
			}
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

func (r *Router) AdminMiddleware() echo.MiddlewareFunc {
	return r.AdminAuthHandler.Middleware()
}

func (r *Router) CORSMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
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
