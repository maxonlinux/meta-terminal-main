package pool

import (
	"sync"

	"github.com/anomalyco/meta-terminal-go/internal/snowflake"
	"github.com/anomalyco/meta-terminal-go/types"
)

var orderPool = sync.Pool{
	New: func() interface{} { return new(types.Order) },
}

var tradePool = sync.Pool{
	New: func() interface{} { return new(types.Trade) },
}

var orderResultPool = sync.Pool{
	New: func() interface{} { return new(types.OrderResult) },
}

func GetOrder() *types.Order {
	return orderPool.Get().(*types.Order)
}

func PutOrder(o *types.Order) {
	if o == nil {
		return
	}
	o.ID = 0
	o.UserID = 0
	o.Symbol = ""
	o.Quantity = 0
	o.Filled = 0
	o.TriggerPrice = 0
	o.Status = 0
	o.Side = 0
	o.Type = 0
	o.TIF = 0
	o.Price = 0
	o.ReduceOnly = false
	o.CloseOnTrigger = false
	o.CreatedAt = 0
	o.UpdatedAt = 0
	o.Prev = 0
	o.Next = 0
	orderPool.Put(o)
}

func GetTrade() *types.Trade {
	return tradePool.Get().(*types.Trade)
}

func PutTrade(t *types.Trade) {
	if t == nil {
		return
	}
	t.ID = 0
	t.Symbol = ""
	t.TakerID = 0
	t.MakerID = 0
	t.TakerOrderID = 0
	t.MakerOrderID = 0
	t.Price = 0
	t.Quantity = 0
	t.ExecutedAt = 0
	tradePool.Put(t)
}

func GetOrderResult() *types.OrderResult {
	return orderResultPool.Get().(*types.OrderResult)
}

func PutOrderResult(r *types.OrderResult) {
	if r == nil {
		return
	}
	r.Order = nil
	r.Trades = nil
	r.Filled = 0
	r.Remaining = 0
	r.Status = 0
	orderResultPool.Put(r)
}

func NextOrderID() types.OrderID {
	return types.OrderID(snowflake.NextID())
}

func NextTradeID() types.TradeID {
	return types.TradeID(snowflake.NextID())
}
