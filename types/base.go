package types

import "time"

type OrderID int64
type TradeID int64
type UserID int64
type Price int64
type Quantity int64

func NanoTime() uint64 {
	return uint64(time.Now().UnixNano())
}
