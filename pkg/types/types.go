package types

import (
	"math/big"
	"time"
)

// Basic type aliases for domain concepts
type OrderID int64
type TradeID int64
type UserID uint64

type Price int64
type Quantity int64

// NowNano возвращает текущее время в наносекундах
// Используется для всех timestamp в системе
func NowNano() uint64 {
	return uint64(time.Now().UnixNano())
}

// Match represents a single match between two orders
type Match struct {
	Trade Trade
	Maker *Order
}

// Trigger represents a conditional order trigger that monitors price levels.
// When triggered, the conditional order is converted to a regular order.
type Trigger struct {
	Order        *Order // The conditional order to activate
	Symbol       string // Symbol being monitored
	Side         int8   // Order side (BUY/SELL)
	TriggerPrice Price  // Price level that triggers the order
}

// Risk
type Risk struct {
	IM  *big.Int // Initial Margin
	MM  *big.Int // Maintenance Margin
	Liq *big.Int // Liquidation Price
}
