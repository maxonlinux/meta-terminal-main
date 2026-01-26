package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/maxonlinux/meta-terminal-go/internal/query"
)

type BalancesHandler struct {
	query *query.Service
}

func NewBalancesHandler(q *query.Service) *BalancesHandler {
	return &BalancesHandler{query: q}
}

func (h *BalancesHandler) List(c echo.Context) error {
	claims := getUser(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "authentication required"})
	}

	balances := h.query.GetBalances(claims.UserID)

	resp := make([]map[string]interface{}, len(balances))
	for i, b := range balances {
		resp[i] = map[string]interface{}{
			"asset":     b.Asset,
			"available": b.Available.String(),
			"locked":    b.Locked.String(),
			"margin":    b.Margin.String(),
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{"balances": resp})
}
