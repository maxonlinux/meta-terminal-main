package types

import (
	"time"
)

type OrderID int64
type UserID int64
type SymbolID int32
type Price int64
type Quantity int64

type Order struct {
	ID            OrderID
	UserID        UserID
	Symbol        SymbolID
	Side          int8
	Type          int8
	TIF           int8
	Status        int8
	Price         Price
	Quantity      Quantity
	Filled        Quantity
	TriggerPrice  Price
	StopOrderType int8

	ReduceOnly     bool
	CloseOnTrigger bool

	// Use uint64 timestamp instead of time.Time to avoid allocations
	CreatedAt uint64
	UpdatedAt uint64

	Next OrderID
	Prev OrderID
}

type Trade struct {
	ID           OrderID
	Symbol       SymbolID
	BuyerID      UserID
	SellerID     UserID
	Price        Price
	Quantity     Quantity
	TakerOrderID OrderID
	MakerOrderID OrderID
	ExecutedAt   uint64 // uint64 instead of time.Time
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
	UserID     UserID
	Symbol     SymbolID
	Size       Quantity
	Side       int8 // -1 = null (нет позиции), 0 = BUY (LONG), 1 = SELL (SHORT)
	EntryPrice Price
	Leverage   int8

	// Риск-параметры (пересчитываются на каждый fill)
	InitialMargin     int64 // IM = size * entryPrice / leverage
	MaintenanceMargin int64 // MM = IM / 10
	LiquidationPrice  Price // Цена ликвидации

	RealizedPnl int64
	Version     int64
}

type OrderInput struct {
	UserID         UserID
	Symbol         SymbolID
	Category       int8
	Side           int8
	Type           int8
	TIF            int8
	Quantity       Quantity
	Price          Price
	TriggerPrice   Price
	StopOrderType  int8
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

// NanoTime returns current time as uint64 nanoseconds
func NanoTime() uint64 {
	return uint64(time.Now().UnixNano())
}
