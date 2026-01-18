package types

type Trade struct {
	ID        TradeID
	MatchID   TradeID
	OrderID   OrderID
	UserID    UserID
	Symbol    string
	Category  int8 // 0=SPOT, 1=LINEAR
	Side      int8
	Price     Price
	Quantity  Quantity
	IsMaker   bool
	Timestamp uint64
}
