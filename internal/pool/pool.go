package pool

import (
	"sync"

	"github.com/anomalyco/meta-terminal-go/internal/types"
)

var orderPool = sync.Pool{
	New: func() interface{} { return &types.Order{} },
}

var tradePool = sync.Pool{
	New: func() interface{} { return &types.Trade{} },
}

var orderResultPool = sync.Pool{
	New: func() interface{} { return &types.OrderResult{} },
}

var tradeSlicePool = sync.Pool{
	New: func() interface{} { return make([]*types.Trade, 0, 8) },
}

func GetOrder() *types.Order {
	return orderPool.Get().(*types.Order)
}

func PutOrder(o *types.Order) {
	*o = types.Order{}
	orderPool.Put(o)
}

func GetTrade() *types.Trade {
	return tradePool.Get().(*types.Trade)
}

func PutTrade(t *types.Trade) {
	*t = types.Trade{}
	tradePool.Put(t)
}

func GetOrderResult() *types.OrderResult {
	return orderResultPool.Get().(*types.OrderResult)
}

func PutOrderResult(r *types.OrderResult) {
	r.Order = nil
	r.Trades = nil
	orderResultPool.Put(r)
}

func GetTradeSlice(capacity int) []*types.Trade {
	s := tradeSlicePool.Get().([]*types.Trade)
	if capacity > cap(s) {
		return make([]*types.Trade, 0, capacity)
	}
	return s[:0]
}

func PutTradeSlice(s []*types.Trade) {
	if s == nil {
		return
	}
	s = s[:0]
	tradeSlicePool.Put(s)
}
