package types

import "github.com/robaho/fixed"

// Basic type aliases for domain concepts
type OrderID int64
type TradeID int64
type FundingID int64
type UserID uint64

type Price = fixed.Fixed
type Quantity = fixed.Fixed

// Match represents a single match between two orders.
type Match struct {
	ID         TradeID
	Symbol     string
	Category   int8
	Price      Price
	Quantity   Quantity
	TakerOrder *Order
	MakerOrder *Order
	Timestamp  uint64
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
	IM  fixed.Fixed // Initial Margin
	MM  fixed.Fixed // Maintenance Margin
	Liq fixed.Fixed // Liquidation Price
}
