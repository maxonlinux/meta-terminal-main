package types

import "time"

type OrderID uint64
type UserID uint64
type TradeID uint64

type Price int64
type Quantity int64

func NowNano() uint64 { return uint64(time.Now().UnixNano()) }

type Order struct {
	ID       OrderID
	UserID   UserID
	Symbol   string
	Category int8

	Side int8
	Type int8
	TIF  int8

	Status int8

	Price    Price
	Quantity Quantity
	Filled   Quantity

	TriggerPrice   Price
	ReduceOnly     bool
	CloseOnTrigger bool
	StopOrderType  int8
	Leverage       int8

	CreatedAt uint64
	UpdatedAt uint64
}

func (o *Order) Remaining() Quantity { return o.Quantity - o.Filled }

type Trade struct {
	ID       TradeID
	Symbol   string
	Category int8

	TakerID      UserID
	MakerID      UserID
	TakerOrderID OrderID
	MakerOrderID OrderID

	Price    Price
	Quantity Quantity

	ExecutedAt uint64
}

type Match struct {
	Trade *Trade
	Maker *Order
}

type OrderInput struct {
	UserID   UserID
	Symbol   string
	Category int8

	Side int8
	Type int8
	TIF  int8

	Quantity Quantity
	Price    Price

	TriggerPrice   Price
	ReduceOnly     bool
	CloseOnTrigger bool
	StopOrderType  int8
	Leverage       int8
}

type OrderResult struct {
	Order     *Order
	Trades    []*Trade
	Filled    Quantity
	Remaining Quantity
	Status    int8
}
