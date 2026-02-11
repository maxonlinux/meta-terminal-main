package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v5"
	"github.com/maxonlinux/meta-terminal-go/internal/engine"
	"github.com/maxonlinux/meta-terminal-go/internal/impersonation"
	"github.com/maxonlinux/meta-terminal-go/internal/kyc"
	"github.com/maxonlinux/meta-terminal-go/internal/persistence"
	"github.com/maxonlinux/meta-terminal-go/internal/plan"
	"github.com/maxonlinux/meta-terminal-go/internal/users"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

type AdminHandler struct {
	plan        *plan.Service
	planRepo    *plan.Repository
	users       *users.Service
	store       *persistence.Store
	kycRepo     *kyc.Repository
	engine      *engine.Engine
	impersonate *impersonation.Service
}

func NewAdminHandler(planService *plan.Service, planRepo *plan.Repository, userService *users.Service, store *persistence.Store, kycRepo *kyc.Repository, eng *engine.Engine, imp *impersonation.Service) *AdminHandler {
	return &AdminHandler{
		plan:        planService,
		planRepo:    planRepo,
		users:       userService,
		store:       store,
		kycRepo:     kycRepo,
		engine:      eng,
		impersonate: imp,
	}
}

type AdminUserPlan struct {
	ID        types.UserID `json:"id"`
	UserID    types.UserID `json:"userId"`
	Plan      string       `json:"plan"`
	IsManual  bool         `json:"isManual"`
	CreatedAt uint64       `json:"createdAt"`
	UpdatedAt uint64       `json:"updatedAt"`
}

type AdminUser struct {
	ID       types.UserID   `json:"id"`
	Email    string         `json:"email"`
	Username string         `json:"username"`
	Phone    string         `json:"phone"`
	Name     *string        `json:"name"`
	Surname  *string        `json:"surname"`
	IsActive bool           `json:"isActive"`
	Plan     *AdminUserPlan `json:"Plan,omitempty"`
}

type AdminUserAddress struct {
	ID      types.UserID `json:"id"`
	Country *string      `json:"country"`
	City    *string      `json:"city"`
	Address *string      `json:"address"`
	Zip     *string      `json:"zip"`
}

type AdminTransaction struct {
	ID          types.FundingID `json:"id"`
	UserID      types.UserID    `json:"userId"`
	Type        string          `json:"type"`
	Status      string          `json:"status"`
	Amount      string          `json:"amount"`
	Destination string          `json:"destination"`
	Message     string          `json:"message"`
	CreatedBy   string          `json:"createdBy"`
	CreatedAt   uint64          `json:"createdAt"`
	UpdatedAt   uint64          `json:"updatedAt"`
}

type AdminFundingUser struct {
	Username string `json:"username"`
}

type AdminFunding struct {
	ID          types.FundingID  `json:"id"`
	UserID      types.UserID     `json:"userId"`
	Type        string           `json:"type"`
	Status      string           `json:"status"`
	Amount      string           `json:"amount"`
	Destination string           `json:"destination"`
	Message     string           `json:"message"`
	CreatedBy   string           `json:"createdBy"`
	CreatedAt   uint64           `json:"createdAt"`
	UpdatedAt   uint64           `json:"updatedAt"`
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
	if h.plan == nil {
		return c.JSON(http.StatusOK, []string{})
	}
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
	if h.users == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "user service unavailable"})
	}
	limit, offset := parsePagination(c)
	query := strings.TrimSpace(c.QueryParam("q"))
	profiles, err := h.users.ListProfiles(limit, offset, query)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load users"})
	}
	res := make([]AdminUser, 0, len(profiles))
	for _, profile := range profiles {
		user := AdminUser{
			ID:       profile.UserID,
			Email:    profile.Email,
			Username: profile.Username,
			Phone:    profile.Phone,
			Name:     profile.Name,
			Surname:  profile.Surname,
			IsActive: profile.IsActive,
		}
		if h.planRepo != nil {
			record, err := h.planRepo.GetUserPlan(profile.UserID)
			if err == nil && record != nil {
				user.Plan = &AdminUserPlan{
					ID:        profile.UserID,
					UserID:    profile.UserID,
					Plan:      record.Plan,
					IsManual:  record.IsManual,
					CreatedAt: record.CreatedAt,
					UpdatedAt: record.UpdatedAt,
				}
			}
		}
		res = append(res, user)
	}
	return c.JSON(http.StatusOK, res)
}

func (h *AdminHandler) User(c *echo.Context) error {
	if h.users == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "user service unavailable"})
	}
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
		ID:       profile.UserID,
		Email:    profile.Email,
		Username: profile.Username,
		Phone:    profile.Phone,
		Name:     profile.Name,
		Surname:  profile.Surname,
		IsActive: profile.IsActive,
	}
	if h.planRepo != nil {
		record, err := h.planRepo.GetUserPlan(profile.UserID)
		if err == nil && record != nil {
			user.Plan = &AdminUserPlan{
				ID:        profile.UserID,
				UserID:    profile.UserID,
				Plan:      record.Plan,
				IsManual:  record.IsManual,
				CreatedAt: record.CreatedAt,
				UpdatedAt: record.UpdatedAt,
			}
		}
	}
	return c.JSON(http.StatusOK, user)
}

func (h *AdminHandler) UserAddress(c *echo.Context) error {
	if h.users == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "user service unavailable"})
	}
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
		ID:      addr.UserID,
		Country: addr.Country,
		City:    addr.City,
		Address: addr.Address,
		Zip:     addr.Zip,
	})
}

func (h *AdminHandler) UserTransactions(c *echo.Context) error {
	if h.store == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "store unavailable"})
	}
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
			ID:          item.ID,
			UserID:      item.UserID,
			Type:        item.Type,
			Status:      item.Status,
			Amount:      item.Amount,
			Destination: item.Destination,
			Message:     item.Message,
			CreatedBy:   item.CreatedBy,
			CreatedAt:   item.CreatedAt,
			UpdatedAt:   item.UpdatedAt,
		})
	}
	return c.JSON(http.StatusOK, res)
}

func (h *AdminHandler) Funding(c *echo.Context) error {
	if h.store == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "store unavailable"})
	}
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
			if err == nil && user != nil {
				username = user.Username
				usernames[item.UserID] = username
			}
		}
		res = append(res, AdminFunding{
			ID:          item.ID,
			UserID:      item.UserID,
			Type:        item.Type,
			Status:      item.Status,
			Amount:      item.Amount,
			Destination: item.Destination,
			Message:     item.Message,
			CreatedBy:   item.CreatedBy,
			CreatedAt:   item.CreatedAt,
			UpdatedAt:   item.UpdatedAt,
			User:        AdminFundingUser{Username: username},
		})
	}
	return c.JSON(http.StatusOK, res)
}

func (h *AdminHandler) ApproveFunding(c *echo.Context) error {
	if h.engine == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "engine unavailable"})
	}
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
	if h.engine == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "engine unavailable"})
	}
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
	if h.store == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "store unavailable"})
	}
	count, err := h.store.CountPendingFundings()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load counts"})
	}
	kycCount := 0
	if h.kycRepo != nil {
		count, err := h.kycRepo.CountPending()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load counts"})
		}
		kycCount = count
	}
	return c.JSON(http.StatusOK, AdminPendingCount{
		Users:        0,
		Wallets:      0,
		Transactions: count,
		KYC:          kycCount,
	})
}

func (h *AdminHandler) Impersonate(c *echo.Context) error {
	if h.impersonate == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "impersonation unavailable"})
	}
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
	if h.plan == nil {
		return c.JSON(http.StatusOK, map[string]interface{}{"current": nil})
	}
	progress, err := h.plan.GetUserPlanProgress(userID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load plan"})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"current":     planNameOrNil(progress.Current),
		"next":        planNameOrNil(progress.Next),
		"remaining":   progress.Remaining,
		"netDeposits": progress.NetDeposits,
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
	if h.plan == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "plan service unavailable"})
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
	if h.plan == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "plan service unavailable"})
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
	parsed, err := strconv.ParseUint(value, 10, 64)
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
