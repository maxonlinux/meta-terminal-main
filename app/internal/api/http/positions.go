package api

import (
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/maxonlinux/meta-terminal-go/internal/engine"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/robaho/fixed"
)

type PositionsHandler struct {
	engine *engine.Engine
}

func NewPositionsHandler(eng *engine.Engine) *PositionsHandler {
	return &PositionsHandler{engine: eng}
}

func (h *PositionsHandler) List(c *echo.Context) error {
	claims := getUser(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "authentication required"})
	}

	positions := h.engine.Portfolio().GetPositions(claims.UserID)

	resp := make([]map[string]interface{}, len(positions))
	for i, p := range positions {
		resp[i] = map[string]interface{}{
			"symbol":     p.Symbol,
			"size":       p.Size.String(),
			"entryPrice": p.EntryPrice.String(),
			"leverage":   p.Leverage.String(),
			"takeProfit": p.TakeProfit.String(),
			"stopLoss":   p.StopLoss.String(),
			"liqPrice":   p.LiqPrice.String(),
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{"positions": resp})
}

type SetLeverageRequest struct {
	Leverage string `json:"leverage"`
}

func (h *PositionsHandler) SetLeverage(c *echo.Context) error {
	claims := getUser(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "authentication required"})
	}

	symbol := c.QueryParam("symbol")
	if symbol == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "symbol is required"})
	}

	var req SetLeverageRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	lev, err := fixed.Parse(req.Leverage)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid leverage"})
	}

	result := h.engine.Cmd(&engine.SetLeverageCmd{
		UserID:   claims.UserID,
		Symbol:   symbol,
		Leverage: types.Leverage(lev),
	})

	if result.Err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": result.Err.Error()})
	}

	return c.NoContent(http.StatusOK)
}

type UpdateTpSlRequest struct {
	TP *string `json:"tp"`
	SL *string `json:"sl"`
}

func (h *PositionsHandler) UpdateTpSl(c *echo.Context) error {
	claims := getUser(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "authentication required"})
	}

	symbol := c.QueryParam("symbol")
	if symbol == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "symbol is required"})
	}

	var req UpdateTpSlRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	var tp types.Price
	var sl types.Price
	if req.TP != nil && *req.TP != "" {
		v, err := fixed.Parse(*req.TP)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid tp"})
		}
		tp = types.Price(v)
	}
	if req.SL != nil && *req.SL != "" {
		v, err := fixed.Parse(*req.SL)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid sl"})
		}
		sl = types.Price(v)
	}

	result := h.engine.Cmd(&engine.UpdateTpSlCmd{
		UserID:     claims.UserID,
		Symbol:     symbol,
		TakeProfit: tp,
		StopLoss:   sl,
	})
	if result.Err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": result.Err.Error()})
	}

	return c.NoContent(http.StatusOK)
}
