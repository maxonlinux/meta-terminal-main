package types

type Order struct {
	ID             OrderID
	UserID         UserID
	Symbol         string
	Side           int8
	Type           int8
	TIF            int8
	Status         int8
	Price          Price
	Quantity       Quantity
	Filled         Quantity
	TriggerPrice   Price
	StopOrderType  int8
	ReduceOnly     bool
	CloseOnTrigger bool
	CreatedAt      uint64
	UpdatedAt      uint64
	Next           OrderID
	Prev           OrderID
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
