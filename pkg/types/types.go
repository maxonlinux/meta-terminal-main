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

// Instrument represents a trading pair with its parameters.
type Instrument struct {
	Symbol     string
	BaseAsset  string
	QuoteAsset string
	PricePrec  int8
	QtyPrec    int8
	MinQty     Quantity
	MaxQty     Quantity
	MinPrice   Price
	MaxPrice   Price
	TickSize   Price
	LotSize    Quantity
}

// Risk
type Risk struct {
	IM  fixed.Fixed // Initial Margin
	MM  fixed.Fixed // Maintenance Margin
	Liq fixed.Fixed // Liquidation Price
}
