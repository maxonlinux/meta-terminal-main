package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v5"
	"github.com/maxonlinux/meta-terminal-go/internal/api/shared"
	"github.com/maxonlinux/meta-terminal-go/internal/engine"
	"github.com/maxonlinux/meta-terminal-go/internal/impersonation"
	"github.com/maxonlinux/meta-terminal-go/internal/kyc"
	"github.com/maxonlinux/meta-terminal-go/internal/otp"
	"github.com/maxonlinux/meta-terminal-go/internal/persistence"
	"github.com/maxonlinux/meta-terminal-go/internal/plan"
	"github.com/maxonlinux/meta-terminal-go/internal/users"
	"github.com/maxonlinux/meta-terminal-go/internal/wallets"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

type AdminHandler struct {
	plan        *plan.Service
	planRepo    *plan.Repository
	wallets     *wallets.Service
	users       *users.Service
	otp         *otp.Service
	store       *persistence.Store
	kycRepo     *kyc.Repository
	engine      *engine.Engine
	impersonate *impersonation.Service
}

func NewAdminHandler(planService *plan.Service, planRepo *plan.Repository, walletService *wallets.Service, userService *users.Service, otpService *otp.Service, store *persistence.Store, kycRepo *kyc.Repository, eng *engine.Engine, imp *impersonation.Service) *AdminHandler {
	return &AdminHandler{
		plan:        planService,
		planRepo:    planRepo,
		wallets:     walletService,
		users:       userService,
		otp:         otpService,
		store:       store,
		kycRepo:     kycRepo,
		engine:      eng,
		impersonate: imp,
	}
}

type AdminUserPlan struct {
	ID        string `json:"id"`
	UserID    string `json:"userId"`
	Plan      string `json:"plan"`
	IsManual  bool   `json:"isManual"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
}

type AdminUser struct {
	ID        string         `json:"id"`
	Email     string         `json:"email"`
	Username  string         `json:"username"`
	Phone     string         `json:"phone"`
	Name      *string        `json:"name"`
	Surname   *string        `json:"surname"`
	IsActive  bool           `json:"isActive"`
	LastLogin int64          `json:"lastLogin"`
	Plan      *AdminUserPlan `json:"Plan,omitempty"`
}

type AdminUserActiveRequest struct {
	Active bool `json:"active"`
}

type AdminUserOTP struct {
	Code      *string `json:"code"`
	ExpiresAt *int64  `json:"expiresAt"`
}

type AdminUserAddress struct {
	ID      string  `json:"id"`
	Country *string `json:"country"`
	City    *string `json:"city"`
	Address *string `json:"address"`
	Zip     *string `json:"zip"`
}

type AdminAddressUpdateRequest struct {
	Country *string `json:"country"`
	City    *string `json:"city"`
	Address *string `json:"address"`
	Zip     *string `json:"zip"`
}

type AdminUserProfileUpdateRequest struct {
	Email   string  `json:"email"`
	Phone   string  `json:"phone"`
	Name    *string `json:"name"`
	Surname *string `json:"surname"`
}

type AdminTransaction struct {
	ID          string `json:"id"`
	UserID      string `json:"userId"`
	Type        string `json:"type"`
	Status      string `json:"status"`
	Amount      string `json:"amount"`
	Destination string `json:"destination"`
	Message     string `json:"message"`
	CreatedBy   string `json:"createdBy"`
	CreatedAt   int64  `json:"createdAt"`
	UpdatedAt   int64  `json:"updatedAt"`
}

type AdminFundingUser struct {
	Username string `json:"username"`
}

type AdminFunding struct {
	ID          string           `json:"id"`
	UserID      string           `json:"userId"`
	Type        string           `json:"type"`
	Status      string           `json:"status"`
	Amount      string           `json:"amount"`
	Destination string           `json:"destination"`
	Message     string           `json:"message"`
	CreatedBy   string           `json:"createdBy"`
	CreatedAt   int64            `json:"createdAt"`
	UpdatedAt   int64            `json:"updatedAt"`
	User        AdminFundingUser `json:"User"`
}

type AdminPendingCount struct {
	Users        int `json:"users"`
	Wallets      int `json:"wallets"`
	Transactions int `json:"transactions"`
	KYC          int `json:"kyc"`
}

type AdminImpersonateResponse struct {
	Code string `json:"code"`
}

func (h *AdminHandler) ExistingPlans(c *echo.Context) error {
	plans := []string{
		string(plan.PlanLowBase),
		string(plan.PlanBase),
		string(plan.PlanStandard),
		string(plan.PlanSilver),
		string(plan.PlanGold),
		string(plan.PlanPlatinum),
		string(plan.PlanAdvanced),
		string(plan.PlanProfessional),
	}
	return c.JSON(http.StatusOK, plans)
}

func (h *AdminHandler) Users(c *echo.Context) error {
	limit, offset := parsePagination(c)
	query := strings.TrimSpace(c.QueryParam("q"))
	profiles, err := h.users.ListProfiles(limit, offset, query)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load users"})
	}
	res := make([]AdminUser, 0, len(profiles))
	for _, profile := range profiles {
		user := AdminUser{
			ID:        strconv.FormatInt(int64(profile.UserID), 10),
			Email:     profile.Email,
			Username:  profile.Username,
			Phone:     profile.Phone,
			Name:      profile.Name,
			Surname:   profile.Surname,
			IsActive:  profile.IsActive,
			LastLogin: shared.UnixMilliFromNano(profile.LastLogin),
		}
		record, err := h.planRepo.GetUserPlan(profile.UserID)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load user plan"})
		}
		if record != nil {
			user.Plan = &AdminUserPlan{
				ID:        strconv.FormatInt(int64(profile.UserID), 10),
				UserID:    strconv.FormatInt(int64(profile.UserID), 10),
				Plan:      record.Plan,
				IsManual:  record.IsManual,
				CreatedAt: int64(record.CreatedAt),
				UpdatedAt: int64(record.UpdatedAt),
			}
		}
		res = append(res, user)
	}
	return c.JSON(http.StatusOK, res)
}

func (h *AdminHandler) User(c *echo.Context) error {
	userID, err := parseUserIDParam(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid user id"})
	}
	profile, err := h.users.GetProfile(userID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load user"})
	}
	if profile == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "user not found"})
	}
	user := AdminUser{
		ID:        strconv.FormatInt(int64(profile.UserID), 10),
		Email:     profile.Email,
		Username:  profile.Username,
		Phone:     profile.Phone,
		Name:      profile.Name,
		Surname:   profile.Surname,
		IsActive:  profile.IsActive,
		LastLogin: shared.UnixMilliFromNano(profile.LastLogin),
	}
	record, err := h.planRepo.GetUserPlan(profile.UserID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load user plan"})
	}
	if record != nil {
		user.Plan = &AdminUserPlan{
			ID:        strconv.FormatInt(int64(profile.UserID), 10),
			UserID:    strconv.FormatInt(int64(profile.UserID), 10),
			Plan:      record.Plan,
			IsManual:  record.IsManual,
			CreatedAt: int64(record.CreatedAt),
			UpdatedAt: int64(record.UpdatedAt),
		}
	}
	return c.JSON(http.StatusOK, user)
}

func (h *AdminHandler) UpdateUserProfile(c *echo.Context) error {
	userID, err := parseUserIDParam(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid user id"})
	}
	var req AdminUserProfileUpdateRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	if strings.TrimSpace(req.Email) == "" || strings.TrimSpace(req.Phone) == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "email and phone are required"})
	}
	if err := h.users.UpdateProfileDetails(userID, req.Email, req.Phone, req.Name, req.Surname); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update user"})
	}
	return c.NoContent(http.StatusOK)
}

func (h *AdminHandler) SetUserActive(c *echo.Context) error {
	userID, err := parseUserIDParam(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid user id"})
	}
	var req AdminUserActiveRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	if err := h.users.SetActive(userID, req.Active); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update user"})
	}
	return c.NoContent(http.StatusOK)
}

func (h *AdminHandler) UserActiveOTP(c *echo.Context) error {
	userID, err := parseUserIDParam(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid user id"})
	}
	user, err := h.users.GetUserByID(userID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load user"})
	}
	if user == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "user not found"})
	}
	code, expiresAt, ok := h.otp.ActiveCode(user.Username)
	if !ok {
		return c.JSON(http.StatusOK, AdminUserOTP{})
	}
	expiresAtMilli := expiresAt.UnixMilli()
	return c.JSON(http.StatusOK, AdminUserOTP{Code: &code, ExpiresAt: &expiresAtMilli})
}

func (h *AdminHandler) UserAddress(c *echo.Context) error {
	userID, err := parseUserIDParam(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid user id"})
	}
	addr, err := h.users.GetAddress(userID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load address"})
	}
	if addr == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "address not found"})
	}
	return c.JSON(http.StatusOK, AdminUserAddress{
		ID:      strconv.FormatInt(int64(addr.UserID), 10),
		Country: addr.Country,
		City:    addr.City,
		Address: addr.Address,
		Zip:     addr.Zip,
	})
}

func (h *AdminHandler) UpdateUserAddress(c *echo.Context) error {
	userID, err := parseUserIDParam(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid user id"})
	}
	var req AdminAddressUpdateRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	addr := users.UserAddress{
		UserID:  userID,
		Country: req.Country,
		City:    req.City,
		Address: req.Address,
		Zip:     req.Zip,
	}
	if err := h.users.UpdateAddress(userID, addr); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update address"})
	}
	return h.UserAddress(c)
}

func (h *AdminHandler) UserTransactions(c *echo.Context) error {
	userID, err := parseUserIDParam(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid user id"})
	}
	limit, offset := parsePagination(c)
	items, err := h.store.ListFundings(userID, limit, offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load transactions"})
	}
	res := make([]AdminTransaction, 0, len(items))
	for _, item := range items {
		res = append(res, AdminTransaction{
			ID:          strconv.FormatInt(int64(item.ID), 10),
			UserID:      strconv.FormatInt(int64(item.UserID), 10),
			Type:        item.Type,
			Status:      item.Status,
			Amount:      item.Amount,
			Destination: item.Destination,
			Message:     item.Message,
			CreatedBy:   item.CreatedBy,
			CreatedAt:   int64(item.CreatedAt),
			UpdatedAt:   int64(item.UpdatedAt),
		})
	}
	return c.JSON(http.StatusOK, res)
}

func (h *AdminHandler) Funding(c *echo.Context) error {
	limit, offset := parsePagination(c)
	query := strings.TrimSpace(c.QueryParam("q"))
	items, err := h.store.ListFundingsAll(limit, offset, query)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load fundings"})
	}
	usernames := make(map[types.UserID]string)
	res := make([]AdminFunding, 0, len(items))
	for _, item := range items {
		username := usernames[item.UserID]
		if username == "" {
			user, err := h.users.GetUserByID(item.UserID)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load user"})
			}
			if user != nil {
				username = user.Username
				usernames[item.UserID] = username
			}
		}
		res = append(res, AdminFunding{
			ID:          strconv.FormatInt(item.ID, 10),
			UserID:      strconv.FormatInt(item.UserID, 10),
			Type:        item.Type,
			Status:      item.Status,
			Amount:      item.Amount,
			Destination: item.Destination,
			Message:     item.Message,
			CreatedBy:   item.CreatedBy,
			CreatedAt:   int64(item.CreatedAt),
			UpdatedAt:   int64(item.UpdatedAt),
			User:        AdminFundingUser{Username: username},
		})
	}
	return c.JSON(http.StatusOK, res)
}

func (h *AdminHandler) ApproveFunding(c *echo.Context) error {
	fundingID, err := parseFundingIDParam(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid funding id"})
	}
	res := h.engine.Cmd(&engine.ApproveFundingCmd{FundingID: fundingID})
	if res.Err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": res.Err.Error()})
	}
	return c.NoContent(http.StatusOK)
}

func (h *AdminHandler) CancelFunding(c *echo.Context) error {
	fundingID, err := parseFundingIDParam(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid funding id"})
	}
	res := h.engine.Cmd(&engine.RejectFundingCmd{FundingID: fundingID})
	if res.Err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": res.Err.Error()})
	}
	return c.NoContent(http.StatusOK)
}

func (h *AdminHandler) PendingCount(c *echo.Context) error {
	count, err := h.store.CountPendingFundings()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load counts"})
	}
	kycCount, err := h.kycRepo.CountPending()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load counts"})
	}
	walletCount := 0
	if count, err := h.wallets.CountWallets(); err == nil {
		walletCount = count
	}
	return c.JSON(http.StatusOK, AdminPendingCount{
		Users:        0,
		Wallets:      walletCount,
		Transactions: count,
		KYC:          kycCount,
	})
}

type WalletRequest struct {
	Name     string `json:"name"`
	Address  string `json:"address"`
	Network  string `json:"network"`
	Currency string `json:"currency"`
	Custom   bool   `json:"custom"`
	Active   bool   `json:"active"`
}

func (h *AdminHandler) ListWallets(c *echo.Context) error {
	items, err := h.wallets.ListWallets()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load wallets"})
	}
	resp := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		resp = append(resp, map[string]interface{}{
			"id":       item.ID,
			"name":     item.Name,
			"address":  item.Address,
			"network":  item.Network,
			"currency": item.Currency,
			"custom":   item.IsCustom,
			"active":   item.IsActive,
			"created":  item.CreatedAt,
			"updated":  item.UpdatedAt,
		})
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *AdminHandler) CreateWallet(c *echo.Context) error {
	var req WalletRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	if req.Name == "" || req.Address == "" || req.Network == "" || req.Currency == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "missing wallet fields"})
	}
	if !req.Active {
		req.Active = true
	}
	_, err := h.wallets.CreateWallet(wallets.Wallet{
		Name:     req.Name,
		Address:  req.Address,
		Network:  req.Network,
		Currency: req.Currency,
		IsCustom: req.Custom,
		IsActive: req.Active,
	})
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.NoContent(http.StatusCreated)
}

func (h *AdminHandler) UpdateWallet(c *echo.Context) error {
	id, err := parseWalletIDParam(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid wallet id"})
	}
	var req WalletRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	if req.Name == "" || req.Address == "" || req.Network == "" || req.Currency == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "missing wallet fields"})
	}
	if err := h.wallets.UpdateWallet(id, wallets.Wallet{
		Name:     req.Name,
		Address:  req.Address,
		Network:  req.Network,
		Currency: req.Currency,
		IsCustom: req.Custom,
		IsActive: req.Active,
	}); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.NoContent(http.StatusOK)
}

type WalletAssignRequest struct {
	WalletID int64 `json:"walletId"`
}

func (h *AdminHandler) AssignUserWallet(c *echo.Context) error {
	userID, err := parseUserIDParam(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid user id"})
	}
	var req WalletAssignRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	if req.WalletID == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "walletId is required"})
	}
	if err := h.wallets.AssignWallet(userID, req.WalletID, "admin"); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.NoContent(http.StatusOK)
}

func (h *AdminHandler) ListUserWallets(c *echo.Context) error {
	userID, err := parseUserIDParam(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid user id"})
	}
	items, err := h.wallets.ListUserWallets(userID, false)
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
			"custom":   item.IsCustom,
			"active":   item.IsActive,
			"assigned": item.AssignedAt,
			"by":       item.AssignedBy,
		})
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *AdminHandler) Impersonate(c *echo.Context) error {
	userID, err := parseUserIDParam(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid user id"})
	}
	code, err := h.impersonate.Create(userID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create impersonation code"})
	}
	return c.JSON(http.StatusOK, AdminImpersonateResponse{Code: code})
}

func (h *AdminHandler) GetUserPlan(c *echo.Context) error {
	userID, err := parseUserIDParam(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid user id"})
	}
	progress, err := h.plan.GetUserPlanProgress(userID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load plan"})
	}
	return c.JSON(http.StatusOK, UserPlanResponse{
		Current:     planNameOrNil(progress.Current),
		Next:        planNameOrNil(progress.Next),
		Remaining:   progress.Remaining.String(),
		NetDeposits: progress.NetDeposits.String(),
	})
}

type PlanUpdateRequest struct {
	Plan string `json:"plan"`
}

func (h *AdminHandler) SetUserPlan(c *echo.Context) error {
	userID, err := parseUserIDParam(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid user id"})
	}
	var req PlanUpdateRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	planName := plan.Name(req.Plan)
	if !isValidPlan(planName) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid plan"})
	}
	if err := h.plan.SetManualPlan(userID, planName); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update plan"})
	}
	return c.NoContent(http.StatusOK)
}

func (h *AdminHandler) ResetUserPlan(c *echo.Context) error {
	userID, err := parseUserIDParam(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid user id"})
	}
	if err := h.plan.ResetManualPlan(userID); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to reset plan"})
	}
	return c.NoContent(http.StatusOK)
}

func parseUserIDParam(value string) (types.UserID, error) {
	if value == "" {
		return 0, strconv.ErrSyntax
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, err
	}
	return types.UserID(parsed), nil
}

func parseFundingIDParam(value string) (types.FundingID, error) {
	if value == "" {
		return 0, strconv.ErrSyntax
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, err
	}
	return types.FundingID(parsed), nil
}

func parseWalletIDParam(value string) (int64, error) {
	if value == "" {
		return 0, strconv.ErrSyntax
	}
	return strconv.ParseInt(value, 10, 64)
}

func parsePagination(c *echo.Context) (int, int) {
	limit := 200
	if raw := c.QueryParam("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	offset := 0
	if raw := c.QueryParam("offset"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 0 {
			offset = parsed
		}
	}
	return limit, offset
}

func isValidPlan(name plan.Name) bool {
	switch name {
	case plan.PlanLowBase,
		plan.PlanBase,
		plan.PlanStandard,
		plan.PlanSilver,
		plan.PlanGold,
		plan.PlanPlatinum,
		plan.PlanAdvanced,
		plan.PlanProfessional:
		return true
	default:
		return false
	}
}
