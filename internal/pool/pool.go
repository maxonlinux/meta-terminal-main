package pool

import (
	"bytes"
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

var matchSlicePool = sync.Pool{
	New: func() interface{} { return make([]types.Match, 0, 8) },
}

var matchPtrSlicePool = sync.Pool{
	New: func() interface{} { return make([]*types.Match, 0, 8) },
}

var bufferPool = sync.Pool{
	New: func() interface{} { return new(bytes.Buffer) },
}

var stringPool = sync.Pool{
	New: func() interface{} { return make([]byte, 0, 32) },
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
	r.Orders = nil
	r.Trades = nil
	for i := range r.OrdersBuf {
		r.OrdersBuf[i] = nil
	}
	for i := range r.TradesBuf {
		r.TradesBuf[i] = types.Trade{}
	}
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

func GetMatchSlice(capacity int) []types.Match {
	s := matchSlicePool.Get().([]types.Match)
	if capacity > cap(s) {
		return make([]types.Match, 0, capacity)
	}
	return s[:0]
}

func PutMatchSlice(s []types.Match) {
	if s == nil {
		return
	}
	s = s[:0]
	matchSlicePool.Put(s)
}

func GetMatch() *[]*types.Match {
	s := matchPtrSlicePool.Get().([]*types.Match)
	return &s
}

func PutMatch(s *[]*types.Match) {
	if s == nil {
		return
	}
	*s = (*s)[:0]
	matchPtrSlicePool.Put(*s)
}

func GetBuffer() *bytes.Buffer {
	return bufferPool.Get().(*bytes.Buffer)
}

func PutBuffer(b *bytes.Buffer) {
	b.Reset()
	bufferPool.Put(b)
}

func GetString() []byte {
	return stringPool.Get().([]byte)
}

func PutString(b []byte) {
	if cap(b) > 64 {
		return
	}
	b = b[:0]
	stringPool.Put(b)
}
