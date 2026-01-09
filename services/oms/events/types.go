package events

import "github.com/anomalyco/meta-terminal-go/internal/id"

const (
	TypeOrderPlaced    = "ORDER_PLACED"
	TypeOrderFilled    = "ORDER_FILLED"
	TypeOrderCancelled = "ORDER_CANCELLED"
	TypeOrderTriggered = "ORDER_TRIGGERED"

	TypeBalanceReserved = "BALANCE_RESERVED"
	TypeBalanceReleased = "BALANCE_RELEASED"
	TypeBalanceDeducted = "BALANCE_DEDUCTED"
	TypeBalanceAdded    = "BALANCE_ADDED"

	TypePositionOpened  = "POSITION_OPENED"
	TypePositionUpdated = "POSITION_UPDATED"
	TypePositionClosed  = "POSITION_CLOSED"

	TypeClearingReserved  = "CLEARING_RESERVED"
	TypeClearingReleased  = "CLEARING_RELEASED"
	TypeClearingTradeExec = "CLEARING_TRADE_EXEC"

	TypeTxCommitted = "TX_COMMITTED"
	TypeTxFailed    = "TX_FAILED"
)

type OrderPlaced struct {
	TxID     id.TxID    `json:"tx_id"`
	OrderID  id.OrderID `json:"order_id"`
	UserID   uint64     `json:"user_id"`
	Symbol   string     `json:"symbol"`
	Category int8       `json:"category"`
	Side     int8       `json:"side"`
	Type     int8       `json:"type"`
	Qty      int64      `json:"qty"`
	Price    int64      `json:"price"`
	TIF      int8       `json:"tif"`
}

type OrderFilled struct {
	TxID    id.TxID    `json:"tx_id"`
	OrderID id.OrderID `json:"order_id"`
	Trades  []Trade    `json:"trades"`
	Filled  int64      `json:"filled"`
	Status  int8       `json:"status"`
}

type Trade struct {
	TradeID      id.TradeID `json:"trade_id"`
	Symbol       string     `json:"symbol"`
	TakerOrderID id.OrderID `json:"taker_order_id"`
	MakerOrderID id.OrderID `json:"maker_order_id"`
	TakerID      uint64     `json:"taker_id"`
	MakerID      uint64     `json:"maker_id"`
	Price        int64      `json:"price"`
	Qty          int64      `json:"qty"`
}

type OrderCancelled struct {
	TxID    id.TxID    `json:"tx_id"`
	OrderID id.OrderID `json:"order_id"`
	Reason  string     `json:"reason"`
}
