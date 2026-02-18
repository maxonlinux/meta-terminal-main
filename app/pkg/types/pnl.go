package types

// RealizedPnL records realized profit and loss for position reductions.
type RealizedPnL struct {
	UserID    UserID
	OrderID   OrderID
	Symbol    string
	Category  int8
	Side      int8
	Price     Price
	Quantity  Quantity
	Realized  Quantity
	Timestamp uint64
}
