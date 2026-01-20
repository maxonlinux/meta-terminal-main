package handlers

import (
	"github.com/labstack/echo/v4"
	"github.com/maxonlinux/meta-terminal-go/internal/engine"
)

type PositionsHandler struct {
	engine *engine.Engine
}

func NewPositionsHandler(eng *engine.Engine) *PositionsHandler {
	return &PositionsHandler{engine: eng}
}

type PositionResponse struct {
	Symbol     string `json:"symbol"`
	Size       string `json:"size"`
	EntryPrice string `json:"entryPrice"`
	Leverage   string `json:"leverage"`
}

func (h *PositionsHandler) GetPositions(c echo.Context) error {
	claims := getUserFromContext(c)
	if claims == nil {
		return Unauthorized(c, "authentication required")
	}

	positions := h.engine.GetPositions(claims.UserID)

	resp := make([]PositionResponse, len(positions))
	for i, p := range positions {
		resp[i] = PositionResponse{
			Symbol:     p.Symbol,
			Size:       p.Size.String(),
			EntryPrice: p.EntryPrice.String(),
			Leverage:   p.Leverage.String(),
		}
	}

	return Success(c, map[string]interface{}{
		"positions": resp,
	})
}

type LeverageRequest struct {
	Leverage string `json:"leverage"`
}

func (h *PositionsHandler) SetLeverage(c echo.Context) error {
	claims := getUserFromContext(c)
	if claims == nil {
		return Unauthorized(c, "authentication required")
	}

	symbol := c.QueryParam("symbol")
	if symbol == "" {
		return BadRequest(c, "symbol is required")
	}

	var req LeverageRequest
	if err := c.Bind(&req); err != nil {
		return BadRequest(c, "invalid request body")
	}

	lev, err := parseFixed(req.Leverage)
	if err != nil {
		return BadRequest(c, "invalid leverage")
	}

	result := h.engine.Cmd(&engine.SetLeverageCmd{
		UserID:   claims.UserID,
		Symbol:   symbol,
		Leverage: lev,
	})

	if result.Err != nil {
		return BadRequest(c, result.Err.Error())
	}

	return Success(c, nil)
}
