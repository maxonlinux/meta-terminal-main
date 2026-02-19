package api

import (
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/maxonlinux/meta-terminal-go/internal/engine"
)

type BalancesHandler struct {
	engine *engine.Engine
}

func NewBalancesHandler(eng *engine.Engine) *BalancesHandler {
	return &BalancesHandler{engine: eng}
}

func (h *BalancesHandler) List(c *echo.Context) error {
	claims := getUser(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "authentication required"})
	}

	balances := h.engine.Portfolio().GetBalances(claims.UserID)

	resp := make([]map[string]interface{}, len(balances))
	for i, b := range balances {
		resp[i] = map[string]interface{}{
			"asset":     b.Asset,
			"available": b.Available.String(),
			"locked":    b.Locked.String(),
			"margin":    b.Margin.String(),
		}
	}

	return c.JSON(http.StatusOK, resp)
}
