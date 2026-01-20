package api

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
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

func (h *PositionsHandler) List(c echo.Context) error {
	claims := getUser(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "authentication required"})
	}

	positions := h.engine.GetPositions(claims.UserID)

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
