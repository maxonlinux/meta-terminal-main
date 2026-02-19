package api

import (
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/maxonlinux/meta-terminal-go/internal/engine"
)

type BalancesHandler struct {
	engine *engine.Engine
}

type BalanceResponse struct {
	Asset     string `json:"asset"`
	Available string `json:"available"`
	Locked    string `json:"locked"`
	Margin    string `json:"margin"`
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

	resp := make([]BalanceResponse, len(balances))
	for i, b := range balances {
		resp[i] = BalanceResponse{
			Asset:     b.Asset,
			Available: b.Available.String(),
			Locked:    b.Locked.String(),
			Margin:    b.Margin.String(),
		}
	}

	return c.JSON(http.StatusOK, resp)
}
