package gateway

import (
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

type OrderRequest struct {
	ClientRequestID *string `json:"clientRequestId"`
	Symbol          string  `json:"symbol"`
	Category        int8    `json:"category"`
	Side            int8    `json:"side"`
	OrderType       int8    `json:"type"`
	TimeInForce     int8    `json:"timeInForce"`
	Quantity        string  `json:"qty"`
	Price           *string `json:"price"`
	TriggerPrice    *string `json:"triggerPrice"`
	ReduceOnly      *bool   `json:"reduceOnly"`
	CloseOnTrigger  *bool   `json:"closeOnTrigger"`
	StopOrderType   *int8   `json:"stopOrderType"`
}

type AmendRequest struct {
	Quantity string `json:"qty"`
}

type LeverageRequest struct {
	Leverage string `json:"leverage"`
}

type RegisterRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type OrderResponse struct {
	ID              types.OrderID `json:"id"`
	ClientRequestID *string       `json:"clientRequestId,omitempty"`
	UserID          types.UserID  `json:"userId"`
	Symbol          string        `json:"symbol"`
	Category        int8          `json:"category"`
	Side            int8          `json:"side"`
	Type            int8          `json:"type"`
	TimeInForce     int8          `json:"timeInForce"`
	Status          int8          `json:"status"`
	Quantity        string        `json:"qty"`
	Filled          string        `json:"filled"`
	Price           string        `json:"price"`
	TriggerPrice    *string       `json:"triggerPrice,omitempty"`
	ReduceOnly      bool          `json:"reduceOnly"`
	CloseOnTrigger  bool          `json:"closeOnTrigger"`
	StopOrderType   *int8         `json:"stopOrderType,omitempty"`
	IsConditional   bool          `json:"isConditional"`
	CreatedAt       uint64        `json:"createdAt"`
	UpdatedAt       uint64        `json:"updatedAt"`
}

type PositionResponse struct {
	Symbol     string `json:"symbol"`
	Size       string `json:"size"`
	EntryPrice string `json:"entryPrice"`
	Leverage   string `json:"leverage"`
}

type BalanceResponse struct {
	Asset     string `json:"asset"`
	Available string `json:"available"`
	Locked    string `json:"locked"`
	Margin    string `json:"margin"`
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

type TradeResponse struct {
	ID        types.TradeID `json:"id"`
	Symbol    string        `json:"symbol"`
	Category  int8          `json:"category"`
	Side      int8          `json:"side"`
	Price     string        `json:"price"`
	Quantity  string        `json:"quantity"`
	IsMaker   bool          `json:"isMaker"`
	Timestamp uint64        `json:"timestamp"`
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

type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
