package api

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/maxonlinux/meta-terminal-go/internal/engine"
	"github.com/maxonlinux/meta-terminal-go/internal/query"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/robaho/fixed"
)

type PositionsHandler struct {
	engine *engine.Engine
	query  *query.Service
}

func NewPositionsHandler(eng *engine.Engine, q *query.Service) *PositionsHandler {
	return &PositionsHandler{engine: eng, query: q}
}

func (h *PositionsHandler) List(c echo.Context) error {
	claims := getUser(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "authentication required"})
	}

	positions := h.query.GetPositions(claims.UserID)

	resp := make([]map[string]interface{}, len(positions))
	for i, p := range positions {
		resp[i] = map[string]interface{}{
			"symbol":     p.Symbol,
			"size":       p.Size.String(),
			"entryPrice": p.EntryPrice.String(),
			"leverage":   p.Leverage.String(),
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{"positions": resp})
}

type SetLeverageRequest struct {
	Leverage string `json:"leverage"`
}

func (h *PositionsHandler) SetLeverage(c echo.Context) error {
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

	lev, err := strconv.ParseFloat(req.Leverage, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid leverage"})
	}

	result := h.engine.Cmd(&engine.SetLeverageCmd{
		UserID:   claims.UserID,
		Symbol:   symbol,
		Leverage: types.Leverage(fixed.NewF(lev)),
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

func (h *PositionsHandler) UpdateTpSl(c echo.Context) error {
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
		v, err := strconv.ParseFloat(*req.TP, 64)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid tp"})
		}
		tp = types.Price(fixed.NewF(v))
	}
	if req.SL != nil && *req.SL != "" {
		v, err := strconv.ParseFloat(*req.SL, 64)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid sl"})
		}
		sl = types.Price(fixed.NewF(v))
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
