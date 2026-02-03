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
