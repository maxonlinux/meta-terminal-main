package actor

import (
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

type MsgPlaceOrder struct {
	UserID types.UserID
	Order  *types.OrderInput
	Result chan<- *types.OrderResult
}

type MsgCancelOrder struct {
	UserID  types.UserID
	OrderID types.OrderID
	Result  chan<- error
}

type MsgPositionUpdate struct {
	UserID  types.UserID
	Symbol  string
	NewSize int64
	NewSide int8
}

type MsgTradeExecuted struct {
	UserID   types.UserID
	Symbol   string
	Category int8
	OrderID  types.OrderID
	Trade    *types.Trade
	Side     int8
}

type MsgTriggerOrder struct {
	UserID types.UserID
	Order  *types.Order
}

type MsgPriceTick struct {
	Symbol string
	Price  types.Price
}

type MsgDeactivateOrder struct {
	UserID  types.UserID
	OrderID types.OrderID
}

type MsgOCOTriggered struct {
	UserID      types.UserID
	TriggeredID types.OrderID
	Symbol      string
}

type MsgGetState struct {
	UserID types.UserID
	Result chan<- interface{}
}

type MsgGetOrder struct {
	UserID  types.UserID
	OrderID types.OrderID
	Result  chan<- *types.Order
}

type MsgGetOrders struct {
	UserID types.UserID
	Result chan<- []*types.Order
}

type MsgMatchingRequest struct {
	Order      *types.Order
	ResultChan chan<- *MatchingResult
}

type MatchingResult struct {
	Trades []types.Trade
	Error  error
}

type MsgGetOrderBook struct {
	Symbol   string
	Category int8
	Result   chan<- interface{}
}

type MsgAddUserOrder struct {
	UserID types.UserID
	Order  *types.Order
}
