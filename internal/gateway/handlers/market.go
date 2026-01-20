package handlers

import (
	"github.com/labstack/echo/v4"
	"github.com/maxonlinux/meta-terminal-go/internal/engine"
)

type MarketHandler struct {
	engine *engine.Engine
}

func NewMarketHandler(eng *engine.Engine) *MarketHandler {
	return &MarketHandler{engine: eng}
}

type InstrumentResponse struct {
	Symbol     string `json:"symbol"`
	BaseAsset  string `json:"baseAsset"`
	QuoteAsset string `json:"quoteAsset"`
	PricePrec  int8   `json:"pricePrec"`
	QtyPrec    int8   `json:"qtyPrec"`
	MinQty     string `json:"minQty"`
	MaxQty     string `json:"maxQty"`
	MinPrice   string `json:"minPrice"`
	MaxPrice   string `json:"maxPrice"`
	TickSize   string `json:"tickSize"`
	LotSize    string `json:"lotSize"`
}

func (h *MarketHandler) GetInstruments(c echo.Context) error {
	symbol := c.QueryParam("symbol")
	instruments := h.engine.GetInstruments(symbol)

	resp := make([]InstrumentResponse, len(instruments))
	for i, inst := range instruments {
		resp[i] = InstrumentResponse{
			Symbol:     inst.Symbol,
			BaseAsset:  inst.BaseAsset,
			QuoteAsset: inst.QuoteAsset,
			PricePrec:  inst.PricePrec,
			QtyPrec:    inst.QtyPrec,
			MinQty:     inst.MinQty.String(),
			MaxQty:     inst.MaxQty.String(),
			MinPrice:   inst.MinPrice.String(),
			MaxPrice:   inst.MaxPrice.String(),
			TickSize:   inst.TickSize.String(),
			LotSize:    inst.LotSize.String(),
		}
	}

	return Success(c, map[string]interface{}{
		"instruments": resp,
	})
}

type BookLevel struct {
	Price string `json:"price"`
	Total string `json:"total"`
}

type OrderBookResponse struct {
	Symbol string      `json:"symbol"`
	Bids   []BookLevel `json:"bids"`
	Asks   []BookLevel `json:"asks"`
}

func (h *MarketHandler) GetOrderBook(c echo.Context) error {
	symbol := c.QueryParam("symbol")
	if symbol == "" {
		return BadRequest(c, "symbol is required")
	}

	book := h.engine.GetOrderBook(symbol)
	if book == nil {
		return NotFound(c, "order book not found")
	}

	return Success(c, book)
}

type TradeResponse struct {
	ID        uint64 `json:"id"`
	Symbol    string `json:"symbol"`
	Category  int8   `json:"category"`
	Side      int8   `json:"side"`
	Price     string `json:"price"`
	Quantity  string `json:"quantity"`
	IsMaker   bool   `json:"isMaker"`
	Timestamp uint64 `json:"timestamp"`
}

func (h *MarketHandler) GetTrades(c echo.Context) error {
	symbol := c.QueryParam("symbol")
	if symbol == "" {
		return BadRequest(c, "symbol is required")
	}

	trades := h.engine.GetPublicTrades(symbol)

	resp := make([]TradeResponse, len(trades))
	for i, t := range trades {
		resp[i] = TradeResponse{
			ID:        uint64(t.ID),
			Symbol:    t.Symbol,
			Category:  t.Category,
			Side:      t.Side,
			Price:     t.Price.String(),
			Quantity:  t.Quantity.String(),
			IsMaker:   t.IsMaker,
			Timestamp: t.Timestamp,
		}
	}

	return Success(c, map[string]interface{}{
		"trades": resp,
	})
}
