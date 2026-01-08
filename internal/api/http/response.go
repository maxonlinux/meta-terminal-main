package httpapi

import (
	"strconv"

	"github.com/anomalyco/meta-terminal-go/internal/types"
)

type OrderResponse struct {
	ID             uint64 `json:"id"`
	UserID         uint64 `json:"userId"`
	Symbol         string `json:"symbol"`
	Category       int8   `json:"category"`
	Side           int8   `json:"side"`
	Type           int8   `json:"type"`
	TIF            int8   `json:"tif"`
	Status         int8   `json:"status"`
	Price          string `json:"price"`
	Quantity       string `json:"qty"`
	Filled         string `json:"filled"`
	Remaining      string `json:"remaining"`
	TriggerPrice   string `json:"triggerPrice"`
	ReduceOnly     bool   `json:"reduceOnly"`
	CloseOnTrigger bool   `json:"closeOnTrigger"`
	StopOrderType  int8   `json:"stopOrderType"`
	Leverage       int8   `json:"leverage"`
	CreatedAt      uint64 `json:"createdAt"`
	UpdatedAt      uint64 `json:"updatedAt"`
}

type TradeResponse struct {
	ID           uint64 `json:"id"`
	Symbol       string `json:"symbol"`
	TakerID      uint64 `json:"takerId"`
	MakerID      uint64 `json:"makerId"`
	TakerOrderID uint64 `json:"takerOrderId"`
	MakerOrderID uint64 `json:"makerOrderId"`
	Price        string `json:"price"`
	Quantity     string `json:"qty"`
	ExecutedAt   uint64 `json:"executedAt"`
}

func orderToResponse(order *types.Order) OrderResponse {
	remaining := int64(0)
	if order != nil {
		remaining = int64(order.Remaining())
	}
	return OrderResponse{
		ID:             uint64(order.ID),
		UserID:         uint64(order.UserID),
		Symbol:         order.Symbol,
		Category:       order.Category,
		Side:           order.Side,
		Type:           order.Type,
		TIF:            order.TIF,
		Status:         order.Status,
		Price:          strconv.FormatInt(int64(order.Price), 10),
		Quantity:       strconv.FormatInt(int64(order.Quantity), 10),
		Filled:         strconv.FormatInt(int64(order.Filled), 10),
		Remaining:      strconv.FormatInt(remaining, 10),
		TriggerPrice:   strconv.FormatInt(int64(order.TriggerPrice), 10),
		ReduceOnly:     order.ReduceOnly,
		CloseOnTrigger: order.CloseOnTrigger,
		StopOrderType:  order.StopOrderType,
		Leverage:       order.Leverage,
		CreatedAt:      order.CreatedAt,
		UpdatedAt:      order.UpdatedAt,
	}
}

func tradeToResponse(trade *types.Trade) TradeResponse {
	return TradeResponse{
		ID:           uint64(trade.ID),
		Symbol:       trade.Symbol,
		TakerID:      uint64(trade.TakerID),
		MakerID:      uint64(trade.MakerID),
		TakerOrderID: uint64(trade.TakerOrderID),
		MakerOrderID: uint64(trade.MakerOrderID),
		Price:        strconv.FormatInt(int64(trade.Price), 10),
		Quantity:     strconv.FormatInt(int64(trade.Quantity), 10),
		ExecutedAt:   trade.ExecutedAt,
	}
}
