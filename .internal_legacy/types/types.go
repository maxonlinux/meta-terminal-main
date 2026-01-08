package types

import "time"

type OrderID uint64
type UserID uint64
type Price int64
type Quantity int64
type TradeID uint64
type SymbolID int32

func NanoTime() uint64 {
	return uint64(time.Now().UnixNano())
}

type Order struct {
	ID             OrderID
	UserID         UserID
	Symbol         string
	Category       int8
	Side           int8
	Type           int8
	TIF            int8
	Status         int8
	Price          Price
	Quantity       Quantity
	Filled         Quantity
	TriggerPrice   Price
	ReduceOnly     bool
	CloseOnTrigger bool
	CreatedAt      uint64
	UpdatedAt      uint64
}

type Trade struct {
	ID           TradeID
	Symbol       string
	TakerID      UserID
	MakerID      UserID
	TakerOrderID OrderID
	MakerOrderID OrderID
	Price        Price
	Quantity     Quantity
	ExecutedAt   uint64
}

type OrderInput struct {
	UserID         UserID
	Symbol         string
	Category       int8
	Side           int8
	Type           int8
	TIF            int8
	Quantity       Quantity
	Price          Price
	TriggerPrice   Price
	ReduceOnly     bool
	CloseOnTrigger bool
}

type OrderResult struct {
	Order     *Order
	Trades    []*Trade
	Filled    Quantity
	Remaining Quantity
	Status    int8
}

type UserBalance struct {
	UserID    UserID
	Asset     string
	Available int64
	Locked    int64
	Margin    int64
	Version   int64
}

type Position struct {
	UserID      UserID
	Symbol      string
	Size        Quantity
	Side        int8
	EntryPrice  Price
	Leverage    int8
	RealizedPnl int64
	Version     int64
}
