package pool

import (
	"bytes"
	"sync"

	"github.com/maxonlinux/meta-terminal-go/pkg/types"
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
	New: func() interface{} {
		s := make([]*types.Trade, 0, 8)
		return &s
	},
}

var matchSlicePool = sync.Pool{
	New: func() interface{} {
		s := make([]types.Match, 0, 8)
		return &s
	},
}

var matchPtrSlicePool = sync.Pool{
	New: func() interface{} {
		s := make([]*types.Match, 0, 8)
		return &s
	},
}

var bufferPool = sync.Pool{
	New: func() interface{} { return new(bytes.Buffer) },
}

var stringPool = sync.Pool{
	New: func() interface{} {
		s := make([]byte, 0, 32)
		return &s
	},
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

func GetTradeSlice(capacity int) *[]*types.Trade {
	s := tradeSlicePool.Get().(*[]*types.Trade)
	if capacity > cap(*s) {
		buf := make([]*types.Trade, 0, capacity)
		*s = buf
		return s
	}
	*s = (*s)[:0]
	return s
}

func PutTradeSlice(s *[]*types.Trade) {
	if s == nil {
		return
	}
	*s = (*s)[:0]
	tradeSlicePool.Put(s)
}

func GetMatchSlice(capacity int) *[]types.Match {
	s := matchSlicePool.Get().(*[]types.Match)
	if capacity > cap(*s) {
		buf := make([]types.Match, 0, capacity)
		*s = buf
		return s
	}
	*s = (*s)[:0]
	return s
}

func PutMatchSlice(s *[]types.Match) {
	if s == nil {
		return
	}
	*s = (*s)[:0]
	matchSlicePool.Put(s)
}

func GetMatch() *[]*types.Match {
	return matchPtrSlicePool.Get().(*[]*types.Match)
}

func PutMatch(s *[]*types.Match) {
	if s == nil {
		return
	}
	*s = (*s)[:0]
	matchPtrSlicePool.Put(s)
}

func GetBuffer() *bytes.Buffer {
	return bufferPool.Get().(*bytes.Buffer)
}

func PutBuffer(b *bytes.Buffer) {
	b.Reset()
	bufferPool.Put(b)
}

func GetString() *[]byte {
	s := stringPool.Get().(*[]byte)
	if s == nil {
		empty := make([]byte, 0, 32)
		return &empty
	}
	return s
}

func PutString(b *[]byte) {
	if b == nil {
		return
	}
	if cap(*b) > 64 {
		return
	}
	*b = (*b)[:0]
	stringPool.Put(b)
}
