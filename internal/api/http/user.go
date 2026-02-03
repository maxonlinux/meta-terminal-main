package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/maxonlinux/meta-terminal-go/internal/engine"
	"github.com/maxonlinux/meta-terminal-go/internal/persistence"
	"github.com/maxonlinux/meta-terminal-go/internal/query"
	"github.com/maxonlinux/meta-terminal-go/internal/users"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/robaho/fixed"
)

type UserHandler struct {
	users *users.Service
	query *query.Service
	eng   *engine.Engine
	store *persistence.Store
}

func NewUserHandler(users *users.Service, queryService *query.Service, eng *engine.Engine, persistenceStore *persistence.Store) *UserHandler {
	return &UserHandler{users: users, query: queryService, eng: eng, store: persistenceStore}
}

func (h *UserHandler) Profile(c echo.Context) error {
	claims := getUser(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "authentication required"})
	}
	profile, err := h.users.GetProfile(claims.UserID)
	if err != nil || profile == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "profile not found"})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"id":       uint64(profile.UserID),
		"email":    profile.Email,
		"username": profile.Username,
		"phone":    profile.Phone,
		"name":     profile.Name,
		"surname":  profile.Surname,
		"isActive": profile.IsActive,
	})
}

type UpdateProfileRequest struct {
	Name    *string `json:"name"`
	Surname *string `json:"surname"`
}

func (h *UserHandler) UpdateProfile(c echo.Context) error {
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

func (h *UserHandler) Settings(c echo.Context) error {
	claims := getUser(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "authentication required"})
	}
	settings, err := h.users.GetSettings(claims.UserID)
	if err != nil || settings == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "settings not found"})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"id":                      uint64(settings.UserID),
		"userId":                  uint64(settings.UserID),
		"is2FAEnabled":            settings.Is2FAEnabled,
		"newsAndOffers":           settings.NewsAndOffers,
		"accessToTransactionData": settings.AccessToTransactionData,
		"accessToGeolocation":     settings.AccessToGeolocation,
		"preferences":             settings.Preferences,
	})
}

type UpdateSettingsRequest struct {
	Is2FAEnabled            *bool           `json:"is2FAEnabled"`
	NewsAndOffers           *bool           `json:"newsAndOffers"`
	AccessToTransactionData *bool           `json:"accessToTransactionData"`
	AccessToGeolocation     *bool           `json:"accessToGeolocation"`
	Preferences             json.RawMessage `json:"preferences"`
}

func (h *UserHandler) UpdateSettings(c echo.Context) error {
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

func (h *UserHandler) Address(c echo.Context) error {
	claims := getUser(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "authentication required"})
	}
	addr, err := h.users.GetAddress(claims.UserID)
	if err != nil || addr == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "address not found"})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"id":      uint64(addr.UserID),
		"userId":  uint64(addr.UserID),
		"country": addr.Country,
		"city":    addr.City,
		"address": addr.Address,
		"zip":     addr.Zip,
	})
}

type UpdateAddressRequest struct {
	Country *string `json:"country"`
	City    *string `json:"city"`
	Address *string `json:"address"`
	Zip     *string `json:"zip"`
}

func (h *UserHandler) UpdateAddress(c echo.Context) error {
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

func (h *UserHandler) UpdatePassword(c echo.Context) error {
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

func (h *UserHandler) Plan(c echo.Context) error {
	claims := getUser(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "authentication required"})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"current":     nil,
		"next":        nil,
		"remaining":   0,
		"netDeposits": 0,
	})
}

func (h *UserHandler) Balances(c echo.Context) error {
	claims := getUser(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "authentication required"})
	}
	balances := h.query.GetBalances(claims.UserID)
	resp := make([]map[string]interface{}, 0, len(balances))
	for _, b := range balances {
		resp = append(resp, map[string]interface{}{
			"currency": b.Asset,
			"free":     b.Available.String(),
			"locked":   b.Locked.String(),
		})
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *UserHandler) Balance(c echo.Context) error {
	claims := getUser(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "authentication required"})
	}
	currency := c.QueryParam("currency")
	if currency == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "currency is required"})
	}
	balance := h.query.GetBalance(claims.UserID, currency)
	if balance == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "balance not found"})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"currency": balance.Asset,
		"free":     balance.Available.String(),
		"locked":   balance.Locked.String(),
	})
}

type FundingRequestBody struct {
	Asset       string `json:"asset"`
	Amount      string `json:"amount"`
	Destination string `json:"destination"`
}

func (h *UserHandler) FundingList(c echo.Context) error {
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
	resp := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		resp = append(resp, map[string]interface{}{
			"id":          uint64(item.ID),
			"userId":      uint64(item.UserID),
			"type":        item.Type,
			"status":      item.Status,
			"asset":       item.Asset,
			"amount":      item.Amount,
			"destination": item.Destination,
			"createdBy":   item.CreatedBy,
			"message":     item.Message,
			"createdAt":   item.CreatedAt,
			"updatedAt":   item.UpdatedAt,
		})
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *UserHandler) FundingDeposit(c echo.Context) error {
	claims := getUser(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "authentication required"})
	}
	var req FundingRequestBody
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	amount, err := strconv.ParseFloat(req.Amount, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid amount"})
	}
	res := h.eng.Cmd(&engine.CreateDepositCmd{UserID: claims.UserID, Asset: req.Asset, Amount: types.Quantity(fixed.NewF(amount)), Destination: req.Destination, CreatedBy: types.FundingCreatedByUser})
	if res.Err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": res.Err.Error()})
	}
	return c.JSON(http.StatusCreated, map[string]string{"message": "FUNDING_CREATED"})
}

func (h *UserHandler) FundingWithdraw(c echo.Context) error {
	claims := getUser(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "authentication required"})
	}
	var req FundingRequestBody
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	amount, err := strconv.ParseFloat(req.Amount, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid amount"})
	}
	res := h.eng.Cmd(&engine.CreateWithdrawalCmd{UserID: claims.UserID, Asset: req.Asset, Amount: types.Quantity(fixed.NewF(amount)), Destination: req.Destination, CreatedBy: types.FundingCreatedByUser})
	if res.Err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": res.Err.Error()})
	}
	return c.JSON(http.StatusCreated, map[string]string{"message": "FUNDING_CREATED"})
}
