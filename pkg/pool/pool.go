package pool

import "sync"

var (
	orderPool = sync.Pool{
		New: func() interface{} {
			return &Order{}
		},
	}
)

type Order struct {
	ID       int64
	UserID   uint64
	Symbol   string
	Category int8
	Side     int8
	Type     int8
	TIF      int8
	Status   int8
	Price    int64
	Quantity int64
	Filled   int64
}

func GetOrder() *Order {
	return orderPool.Get().(*Order)
}

func PutOrder(o *Order) {
	orderPool.Put(o)
}
