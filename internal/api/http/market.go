package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/maxonlinux/meta-terminal-go/internal/api/shared"
	"github.com/maxonlinux/meta-terminal-go/internal/engine"
	orderbook "github.com/maxonlinux/meta-terminal-go/internal/orderbook"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
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

func (h *MarketHandler) Instruments(c echo.Context) error {
	symbol := c.QueryParam("symbol")
	var instruments []*types.Instrument
	if symbol != "" {
		inst := h.engine.Registry().GetInstrument(symbol)
		if inst != nil {
			instruments = []*types.Instrument{inst}
		} else {
			instruments = []*types.Instrument{}
		}
	} else {
		instruments = h.engine.Registry().GetInstruments()
	}

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

	return c.JSON(http.StatusOK, map[string]interface{}{"instruments": resp})
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

func (h *MarketHandler) OrderBook(c echo.Context) error {
	symbol := c.QueryParam("symbol")
	categoryParam := c.QueryParam("category")
	if symbol == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "symbol is required"})
	}
	if categoryParam == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "category is required"})
	}
	category, err := shared.ParseCategoryParam(categoryParam)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	book := h.engine.ReadBook(category, symbol)
	if book == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "order book not found"})
	}

	snap := snapshotBook(book, 0)
	resp := OrderBookResponse{Symbol: symbol}
	for _, bid := range snap.Bids {
		resp.Bids = append(resp.Bids, BookLevel{Price: bid.Price.String(), Total: bid.Total.String()})
	}
	for _, ask := range snap.Asks {
		resp.Asks = append(resp.Asks, BookLevel{Price: ask.Price.String(), Total: ask.Total.String()})
	}

	return c.JSON(http.StatusOK, resp)
}

type TradeResponse struct {
	ID        uint64 `json:"id"`
	Symbol    string `json:"symbol"`
	Category  string `json:"category"`
	Side      string `json:"side"`
	Price     string `json:"price"`
	Quantity  string `json:"quantity"`
	IsMaker   bool   `json:"isMaker"`
	Timestamp uint64 `json:"timestamp"`
}

func (h *MarketHandler) Trades(c echo.Context) error {
	symbol := c.QueryParam("symbol")
	categoryParam := c.QueryParam("category")
	if symbol == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "symbol is required"})
	}
	if categoryParam == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "category is required"})
	}
	category, err := shared.ParseCategoryParam(categoryParam)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	trades := h.engine.TradeFeed().Recent(category, symbol)

	resp := make([]TradeResponse, len(trades))
	for i, t := range trades {
		resp[i] = TradeResponse{
			ID:        uint64(t.ID),
			Symbol:    t.Symbol,
			Category:  shared.CategoryToString(t.Category),
			Side:      shared.SideToString(t.Side),
			Price:     t.Price.String(),
			Quantity:  t.Quantity.String(),
			IsMaker:   t.IsMaker,
			Timestamp: t.Timestamp,
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{"trades": resp})
}

func snapshotBook(book *orderbook.OrderBook, depth int) orderbook.Snapshot {
	if depth <= 0 {
		depth = 50
	}
	return book.Snapshot(depth)
}
