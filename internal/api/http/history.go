package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/maxonlinux/meta-terminal-go/internal/api/shared"
	"github.com/maxonlinux/meta-terminal-go/internal/persistence"
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
)

type HistoryHandler struct {
	store *persistence.Store
}

func NewHistoryHandler(store *persistence.Store) *HistoryHandler {
	return &HistoryHandler{store: store}
}

func (h *HistoryHandler) Orders(c echo.Context) error {
	claims := getUser(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "authentication required"})
	}

	symbol := c.QueryParam("symbol")
	categoryParam := c.QueryParam("category")
	limit, err := parseLimit(c.QueryParam("limit"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	offset, err := parseOffset(c.QueryParam("offset"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	var category *int8
	if categoryParam != "" {
		parsed, err := shared.ParseCategoryParam(categoryParam)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		category = &parsed
	}

	orders, err := h.store.ListOrders(claims.UserID, symbol, category, limit, offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load orders"})
	}

	resp := make([]shared.OrderResponse, 0, len(orders))
	for _, order := range orders {
		if order.Origin == constants.ORDER_ORIGIN_SYSTEM {
			continue
		}
		resp = append(resp, shared.OrderResponseFromRecord(order))
	}

	return c.JSON(http.StatusOK, map[string]interface{}{"orders": resp})
}

func (h *HistoryHandler) Fills(c echo.Context) error {
	claims := getUser(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "authentication required"})
	}

	symbol := c.QueryParam("symbol")
	categoryParam := c.QueryParam("category")
	limit, err := parseLimit(c.QueryParam("limit"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	offset, err := parseOffset(c.QueryParam("offset"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	var category *int8
	if categoryParam != "" {
		parsed, err := shared.ParseCategoryParam(categoryParam)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		category = &parsed
	}

	fills, err := h.store.ListFills(claims.UserID, symbol, category, limit, offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load fills"})
	}

	resp := make([]shared.FillResponse, 0, len(fills))
	for _, fill := range fills {
		resp = append(resp, shared.FillResponseFromRecord(fill))
	}

	return c.JSON(http.StatusOK, map[string]interface{}{"fills": resp})
}

type PnLResponse struct {
	// PnLResponse is a single realized PnL record for the user.
	ID        int64  `json:"id"`
	OrderID   uint64 `json:"orderId"`
	Symbol    string `json:"symbol"`
	Category  int8   `json:"category"`
	Side      int8   `json:"side"`
	Price     string `json:"price"`
	Quantity  string `json:"qty"`
	Realized  string `json:"realized"`
	CreatedAt uint64 `json:"createdAt"`
}

func (h *HistoryHandler) PnL(c echo.Context) error {
	// Returns realized PnL history for the authenticated user.
	claims := getUser(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "authentication required"})
	}

	symbol := c.QueryParam("symbol")
	categoryParam := c.QueryParam("category")
	limit, err := parseLimit(c.QueryParam("limit"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	offset, err := parseOffset(c.QueryParam("offset"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	var category *int8
	if categoryParam != "" {
		parsed, err := shared.ParseCategoryParam(categoryParam)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		category = &parsed
	}

	items, err := h.store.ListRPNL(claims.UserID, symbol, category, limit, offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load pnl"})
	}

	resp := make([]PnLResponse, 0, len(items))
	for _, item := range items {
		resp = append(resp, PnLResponse{
			ID:        item.ID,
			OrderID:   uint64(item.OrderID),
			Symbol:    item.Symbol,
			Category:  item.Category,
			Side:      item.Side,
			Price:     item.Price,
			Quantity:  item.Quantity,
			Realized:  item.Realized,
			CreatedAt: item.CreatedAt,
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{"pnl": resp})
}
