package handlers

import (
	"github.com/labstack/echo/v4"
	"github.com/maxonlinux/meta-terminal-go/internal/engine"
)

type BalancesHandler struct {
	engine *engine.Engine
}

func NewBalancesHandler(eng *engine.Engine) *BalancesHandler {
	return &BalancesHandler{engine: eng}
}

type BalanceResponse struct {
	Asset     string `json:"asset"`
	Available string `json:"available"`
	Locked    string `json:"locked"`
	Margin    string `json:"margin"`
}

func (h *BalancesHandler) GetBalances(c echo.Context) error {
	claims := getUserFromContext(c)
	if claims == nil {
		return Unauthorized(c, "authentication required")
	}

	balances := h.engine.GetBalances(claims.UserID)

	resp := make([]BalanceResponse, len(balances))
	for i, b := range balances {
		resp[i] = BalanceResponse{
			Asset:     b.Asset,
			Available: b.Available.String(),
			Locked:    b.Locked.String(),
			Margin:    b.Margin.String(),
		}
	}

	return Success(c, map[string]interface{}{
		"balances": resp,
	})
}
