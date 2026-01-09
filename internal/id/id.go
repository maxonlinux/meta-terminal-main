package id

import (
	"sync/atomic"
	"time"
)

type TxID uint64
type OrderID uint64
type TradeID uint64
type UserID uint64
type PositionID uint64
type BalanceID uint64

func NewTxID() TxID {
	return TxID(generate())
}

func NewOrderID() OrderID {
	return OrderID(generate())
}

func NewTradeID() TradeID {
	return TradeID(generate())
}

func NewUserID() UserID {
	return UserID(generate())
}

func NewPositionID() PositionID {
	return PositionID(generate())
}

func NewBalanceID() BalanceID {
	return BalanceID(generate())
}

func NodeID() uint64 {
	return generate() % 10000
}

var counter uint64 = 0

func generate() uint64 {
	return atomic.AddUint64(&counter, 1)
}

func init() {
	counter = uint64(time.Now().UnixNano())
}
