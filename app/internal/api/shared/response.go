package shared

import (
	"github.com/maxonlinux/meta-terminal-go/internal/persistence"
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

type OrderResponse struct {
	ID             uint64 `json:"id"`
	UserID         uint64 `json:"userId"`
	Symbol         string `json:"symbol"`
	Category       string `json:"category"`
	Origin         string `json:"origin"`
	Side           string `json:"side"`
	Type           string `json:"type"`
	TimeInForce    string `json:"timeInForce"`
	Status         string `json:"status"`
	Qty            string `json:"qty"`
	Filled         string `json:"filled"`
	Price          string `json:"price"`
	TriggerPrice   string `json:"triggerPrice"`
	ReduceOnly     bool   `json:"reduceOnly"`
	CloseOnTrigger bool   `json:"closeOnTrigger"`
	StopOrderType  string `json:"stopOrderType,omitempty"`
	IsConditional  bool   `json:"isConditional"`
	CreatedAt      uint64 `json:"createdAt"`
	UpdatedAt      uint64 `json:"updatedAt"`
}

func OrderResponseFromOrder(o *types.Order) OrderResponse {
	resp := OrderResponse{
		ID:             uint64(o.ID),
		UserID:         uint64(o.UserID),
		Symbol:         o.Symbol,
		Category:       CategoryToString(o.Category),
		Origin:         OriginToString(o.Origin),
		Side:           SideToString(o.Side),
		Type:           OrderTypeToString(o.Type),
		TimeInForce:    TifToString(o.TIF),
		Status:         OrderStatusToString(o.Status),
		Qty:            o.Quantity.String(),
		Filled:         o.Filled.String(),
		Price:          o.Price.String(),
		ReduceOnly:     o.ReduceOnly,
		CloseOnTrigger: o.CloseOnTrigger,
		IsConditional:  o.IsConditional,
		CreatedAt:      UnixMilliFromNano(o.CreatedAt),
		UpdatedAt:      UnixMilliFromNano(o.UpdatedAt),
	}
	if o.Type == constants.ORDER_TYPE_MARKET {
		resp.Price = ""
	}

	if o.TriggerPrice.Sign() > 0 {
		resp.TriggerPrice = o.TriggerPrice.String()
	} else {
		resp.TriggerPrice = ""
	}

	if o.StopOrderType != 0 {
		resp.StopOrderType = StopOrderTypeToString(o.StopOrderType)
	}

	return resp
}

func OrderResponseFromRecord(order persistence.OrderRecord) OrderResponse {
	resp := OrderResponse{
		ID:             uint64(order.ID),
		UserID:         uint64(order.UserID),
		Symbol:         order.Symbol,
		Category:       CategoryToString(order.Category),
		Origin:         OriginToString(order.Origin),
		Side:           SideToString(order.Side),
		Type:           OrderTypeToString(order.Type),
		TimeInForce:    TifToString(order.TIF),
		Status:         OrderStatusToString(order.Status),
		Qty:            order.Qty,
		Filled:         order.Filled,
		Price:          order.Price,
		TriggerPrice:   order.TriggerPrice,
		ReduceOnly:     order.ReduceOnly,
		CloseOnTrigger: order.CloseOnTrigger,
		IsConditional:  order.IsConditional,
		CreatedAt:      UnixMilliFromNano(order.CreatedAt),
		UpdatedAt:      UnixMilliFromNano(order.UpdatedAt),
	}
	if order.Type == constants.ORDER_TYPE_MARKET {
		resp.Price = ""
	}

	if order.StopOrderType != 0 {
		resp.StopOrderType = StopOrderTypeToString(order.StopOrderType)
	}

	return resp
}

type FillResponse struct {
	ID                  uint64 `json:"id"`
	UserID              uint64 `json:"userId"`
	OrderID             uint64 `json:"orderId"`
	CounterpartyOrderID uint64 `json:"counterpartyOrderId"`
	Symbol              string `json:"symbol"`
	Category            string `json:"category"`
	Side                string `json:"side"`
	Role                string `json:"role"`
	Price               string `json:"price"`
	Qty                 string `json:"qty"`
	Timestamp           uint64 `json:"timestamp"`
	OrderType           string `json:"orderType"`
}

func FillResponseFromRecord(fill persistence.FillRecord) FillResponse {
	return FillResponse{
		ID:                  uint64(fill.ID),
		UserID:              uint64(fill.UserID),
		OrderID:             uint64(fill.OrderID),
		CounterpartyOrderID: uint64(fill.CounterpartyOrderID),
		Symbol:              fill.Symbol,
		Category:            CategoryToString(fill.Category),
		Side:                SideToString(fill.Side),
		Role:                fill.Role,
		Price:               fill.Price,
		Qty:                 fill.Qty,
		Timestamp:           UnixMilliFromNano(fill.Timestamp),
		OrderType:           OrderTypeToString(fill.OrderType),
	}
}
