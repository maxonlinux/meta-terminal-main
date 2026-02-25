package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v5"
	"github.com/maxonlinux/meta-terminal-go/internal/api/shared"
	"github.com/maxonlinux/meta-terminal-go/internal/engine"
	"github.com/maxonlinux/meta-terminal-go/internal/persistence"
	"github.com/maxonlinux/meta-terminal-go/internal/plan"
	"github.com/maxonlinux/meta-terminal-go/internal/users"
	"github.com/maxonlinux/meta-terminal-go/internal/wallets"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/robaho/fixed"
)

type UserHandler struct {
	users   *users.Service
	eng     *engine.Engine
	store   *persistence.Store
	plan    *plan.Service
	wallets *wallets.Service
}

type UserProfileResponse struct {
	ID        string  `json:"id"`
	Email     string  `json:"email"`
	Username  string  `json:"username"`
	Phone     string  `json:"phone"`
	Name      *string `json:"name"`
	Surname   *string `json:"surname"`
	IsActive  bool    `json:"isActive"`
	LastLogin int64   `json:"lastLogin"`
}

type UserSettingsResponse struct {
	ID                      string `json:"id"`
	UserID                  string `json:"userId"`
	Is2FAEnabled            bool   `json:"is2FAEnabled"`
	NewsAndOffers           bool   `json:"newsAndOffers"`
	AccessToTransactionData bool   `json:"accessToTransactionData"`
	AccessToGeolocation     bool   `json:"accessToGeolocation"`
	Preferences             string `json:"preferences"`
}

type UserAddressResponse struct {
	ID      string  `json:"id"`
	UserID  string  `json:"userId"`
	Country *string `json:"country"`
	City    *string `json:"city"`
	Address *string `json:"address"`
	Zip     *string `json:"zip"`
}

type UserPlanResponse struct {
	Current     interface{} `json:"current"`
	Next        interface{} `json:"next"`
	Remaining   string      `json:"remaining"`
	NetDeposits string      `json:"netDeposits"`
}

func NewUserHandler(users *users.Service, eng *engine.Engine, persistenceStore *persistence.Store, planService *plan.Service, walletService *wallets.Service) *UserHandler {
	return &UserHandler{users: users, eng: eng, store: persistenceStore, plan: planService, wallets: walletService}
}

func (h *UserHandler) Profile(c *echo.Context) error {
	claims := getUser(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "authentication required"})
	}
	profile, err := h.users.GetProfile(claims.UserID)
	if err != nil || profile == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "profile not found"})
	}
	return c.JSON(http.StatusOK, UserProfileResponse{
		ID:        strconv.FormatInt(int64(profile.UserID), 10),
		Email:     profile.Email,
		Username:  profile.Username,
		Phone:     profile.Phone,
		Name:      profile.Name,
		Surname:   profile.Surname,
		IsActive:  profile.IsActive,
		LastLogin: shared.UnixMilliFromNano(profile.LastLogin),
	})
}

type UpdateProfileRequest struct {
	Name    *string `json:"name"`
	Surname *string `json:"surname"`
}

func (h *UserHandler) UpdateProfile(c *echo.Context) error {
	claims := getUser(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "authentication required"})
	}
	var req UpdateProfileRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	if err := h.users.UpdateProfile(claims.UserID, req.Name, req.Surname); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update profile"})
	}
	return h.Profile(c)
}

func (h *UserHandler) Settings(c *echo.Context) error {
	claims := getUser(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "authentication required"})
	}
	settings, err := h.users.GetSettings(claims.UserID)
	if err != nil || settings == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "settings not found"})
	}
	return c.JSON(http.StatusOK, UserSettingsResponse{
		ID:                      strconv.FormatInt(int64(settings.UserID), 10),
		UserID:                  strconv.FormatInt(int64(settings.UserID), 10),
		Is2FAEnabled:            settings.Is2FAEnabled,
		NewsAndOffers:           settings.NewsAndOffers,
		AccessToTransactionData: settings.AccessToTransactionData,
		AccessToGeolocation:     settings.AccessToGeolocation,
		Preferences:             settings.Preferences,
	})
}

type UpdateSettingsRequest struct {
	Is2FAEnabled            *bool           `json:"is2FAEnabled"`
	NewsAndOffers           *bool           `json:"newsAndOffers"`
	AccessToTransactionData *bool           `json:"accessToTransactionData"`
	AccessToGeolocation     *bool           `json:"accessToGeolocation"`
	Preferences             json.RawMessage `json:"preferences"`
}

func (h *UserHandler) UpdateSettings(c *echo.Context) error {
	claims := getUser(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "authentication required"})
	}
	var req UpdateSettingsRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	current, err := h.users.GetSettings(claims.UserID)
	if err != nil || current == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "settings not found"})
	}
	if req.Is2FAEnabled != nil {
		current.Is2FAEnabled = *req.Is2FAEnabled
	}
	if req.NewsAndOffers != nil {
		current.NewsAndOffers = *req.NewsAndOffers
	}
	if req.AccessToTransactionData != nil {
		current.AccessToTransactionData = *req.AccessToTransactionData
	}
	if req.AccessToGeolocation != nil {
		current.AccessToGeolocation = *req.AccessToGeolocation
	}
	if len(req.Preferences) > 0 {
		current.Preferences = string(req.Preferences)
	}
	if err := h.users.UpdateSettings(claims.UserID, *current); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update settings"})
	}
	return h.Settings(c)
}

func (h *UserHandler) Address(c *echo.Context) error {
	claims := getUser(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "authentication required"})
	}
	addr, err := h.users.GetAddress(claims.UserID)
	if err != nil || addr == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "address not found"})
	}
	return c.JSON(http.StatusOK, UserAddressResponse{
		ID:      strconv.FormatInt(int64(addr.UserID), 10),
		UserID:  strconv.FormatInt(int64(addr.UserID), 10),
		Country: addr.Country,
		City:    addr.City,
		Address: addr.Address,
		Zip:     addr.Zip,
	})
}

type UpdateAddressRequest struct {
	Country *string `json:"country"`
	City    *string `json:"city"`
	Address *string `json:"address"`
	Zip     *string `json:"zip"`
}

func (h *UserHandler) UpdateAddress(c *echo.Context) error {
	claims := getUser(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "authentication required"})
	}
	var req UpdateAddressRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	addr := users.UserAddress{
		UserID:  claims.UserID,
		Country: req.Country,
		City:    req.City,
		Address: req.Address,
		Zip:     req.Zip,
	}
	if err := h.users.UpdateAddress(claims.UserID, addr); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update address"})
	}
	return h.Address(c)
}

type UpdatePasswordRequest struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

func (h *UserHandler) UpdatePassword(c *echo.Context) error {
	claims := getUser(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "authentication required"})
	}
	var req UpdatePasswordRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	user, err := h.users.GetUserByID(claims.UserID)
	if err != nil || user == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "user not found"})
	}
	if !h.users.ValidatePassword(user, req.OldPassword) {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "invalid password"})
	}
	hash, err := users.HashPassword(req.NewPassword)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update password"})
	}
	if err := h.users.UpdatePassword(claims.UserID, hash); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update password"})
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "PASSWORD_UPDATED"})
}

func (h *UserHandler) Plan(c *echo.Context) error {
	claims := getUser(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "authentication required"})
	}
	progress, err := h.plan.GetUserPlanProgress(claims.UserID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load plan"})
	}
	current := planNameOrNil(progress.Current)
	next := planNameOrNil(progress.Next)
	// Serialize fixed-point values as strings to preserve precision.
	return c.JSON(http.StatusOK, UserPlanResponse{
		Current:     current,
		Next:        next,
		Remaining:   progress.Remaining.String(),
		NetDeposits: progress.NetDeposits.String(),
	})
}

func planNameOrNil(name plan.Name) interface{} {
	if name == "" {
		return nil
	}
	return string(name)
}

func (h *UserHandler) Balances(c *echo.Context) error {
	claims := getUser(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "authentication required"})
	}
	balances := h.eng.Portfolio().GetBalances(claims.UserID)
	resp := make([]BalanceResponse, 0, len(balances))
	for _, b := range balances {
		resp = append(resp, BalanceResponse{
			Asset:     b.Asset,
			Available: b.Available.String(),
			Locked:    b.Locked.String(),
			Margin:    b.Margin.String(),
		})
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *UserHandler) Balance(c *echo.Context) error {
	claims := getUser(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "authentication required"})
	}
	asset := c.QueryParam("asset")
	if asset == "" {
		asset = c.QueryParam("currency")
	}
	if asset == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "asset is required"})
	}
	balance := h.eng.Portfolio().GetBalance(claims.UserID, asset)
	if balance == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "balance not found"})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"asset":     balance.Asset,
		"available": balance.Available.String(),
		"locked":    balance.Locked.String(),
		"margin":    balance.Margin.String(),
	})
}

type FundingRequestBody struct {
	Asset       string `json:"asset"`
	Amount      string `json:"amount"`
	Destination string `json:"destination"`
}

type DepositRequestBody struct {
	WalletID int64  `json:"walletId"`
	Amount   string `json:"amount"`
}

type FundingResponse struct {
	ID          string `json:"id"`
	UserID      string `json:"userId"`
	Type        string `json:"type"`
	Status      string `json:"status"`
	Asset       string `json:"asset"`
	Amount      string `json:"amount"`
	Destination string `json:"destination"`
	CreatedBy   string `json:"createdBy"`
	Message     string `json:"message"`
	CreatedAt   int64  `json:"createdAt"`
	UpdatedAt   int64  `json:"updatedAt"`
}

func (h *UserHandler) FundingList(c *echo.Context) error {
	claims := getUser(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "authentication required"})
	}
	limit, err := parseLimit(c.QueryParam("limit"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid limit"})
	}
	offset, err := parseOffset(c.QueryParam("offset"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid offset"})
	}
	items, err := h.store.ListFundings(claims.UserID, limit, offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load funding"})
	}
	resp := make([]FundingResponse, 0, len(items))
	for _, item := range items {
		resp = append(resp, FundingResponse{
			ID:          strconv.FormatInt(item.ID, 10),
			UserID:      strconv.FormatInt(item.UserID, 10),
			Type:        item.Type,
			Status:      item.Status,
			Asset:       item.Asset,
			Amount:      item.Amount,
			Destination: item.Destination,
			CreatedBy:   item.CreatedBy,
			Message:     item.Message,
			CreatedAt:   shared.UnixMilliFromNano(item.CreatedAt),
			UpdatedAt:   shared.UnixMilliFromNano(item.UpdatedAt),
		})
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *UserHandler) FundingDeposit(c *echo.Context) error {
	claims := getUser(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "authentication required"})
	}
	var req DepositRequestBody
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	amount, err := fixed.Parse(req.Amount)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid amount"})
	}
	if req.WalletID == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "walletId is required"})
	}
	wallet, err := h.wallets.GetUserWallet(claims.UserID, req.WalletID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load wallet"})
	}
	if wallet == nil || !wallet.IsActive {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "wallet not available"})
	}
	res := h.eng.Cmd(&engine.CreateDepositCmd{UserID: claims.UserID, Asset: wallet.Currency, Amount: types.Quantity(amount), Destination: wallet.Address, CreatedBy: types.FundingCreatedByUser})
	if res.Err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": res.Err.Error()})
	}
	return c.JSON(http.StatusCreated, map[string]string{"message": "FUNDING_CREATED"})
}

func (h *UserHandler) Wallets(c *echo.Context) error {
	claims := getUser(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "authentication required"})
	}
	items, err := h.wallets.ListUserWallets(claims.UserID, true)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load wallets"})
	}
	resp := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		resp = append(resp, map[string]interface{}{
			"id":       item.WalletID,
			"name":     item.Name,
			"address":  item.Address,
			"network":  item.Network,
			"currency": item.Currency,
		})
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *UserHandler) FundingWithdraw(c *echo.Context) error {
	claims := getUser(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "authentication required"})
	}
	var req FundingRequestBody
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	amount, err := fixed.Parse(req.Amount)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid amount"})
	}
	res := h.eng.Cmd(&engine.CreateWithdrawalCmd{UserID: claims.UserID, Asset: req.Asset, Amount: types.Quantity(amount), Destination: req.Destination, CreatedBy: types.FundingCreatedByUser})
	if res.Err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": res.Err.Error()})
	}
	return c.JSON(http.StatusCreated, map[string]string{"message": "FUNDING_CREATED"})
}
