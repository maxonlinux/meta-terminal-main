package memory

import (
	"sync"
	"sync/atomic"

	"github.com/anomalyco/meta-terminal-go/internal/types"
)

type OrderPool struct {
	pool sync.Pool
}

func NewOrderPool() *OrderPool {
	return &OrderPool{
		pool: sync.Pool{
			New: func() interface{} {
				return &types.Order{}
			},
		},
	}
}

func (p *OrderPool) Get() *types.Order {
	return p.pool.Get().(*types.Order)
}

func (p *OrderPool) Put(o *types.Order) {
	if o == nil {
		return
	}
	o.ID = 0
	o.UserID = 0
	o.Symbol = 0
	o.Quantity = 0
	o.Filled = 0
	o.TriggerPrice = 0
	o.Status = 0
	o.Prev = 0
	o.Next = 0
	p.pool.Put(o)
}

type TradePool struct {
	pool    sync.Pool
	counter int64
}

func NewTradePool() *TradePool {
	return &TradePool{
		pool: sync.Pool{
			New: func() interface{} {
				return &types.Trade{}
			},
		},
	}
}

func (p *TradePool) Get() *types.Trade {
	return p.pool.Get().(*types.Trade)
}

func (p *TradePool) Put(t *types.Trade) {
	if t == nil {
		return
	}
	t.BuyerID = 0
	t.SellerID = 0
	t.Quantity = 0
	t.Price = 0
	t.TakerOrderID = 0
	t.MakerOrderID = 0
	p.pool.Put(t)
}

type TradeBuffer struct {
	trades []*types.Trade
	pool   *TradePool
	count  int
}

func NewTradeBuffer(pool *TradePool, cap int) *TradeBuffer {
	return &TradeBuffer{
		trades: make([]*types.Trade, 0, cap),
		pool:   pool,
	}
}

func (b *TradeBuffer) Add(trade *types.Trade) {
	if b.count < cap(b.trades) {
		b.trades = append(b.trades, trade)
		b.count++
	}
}

func (b *TradeBuffer) Slice() []*types.Trade {
	return b.trades[:b.count]
}

func (b *TradeBuffer) Reset() {
	for i := 0; i < b.count; i++ {
		b.pool.Put(b.trades[i])
	}
	b.trades = b.trades[:0]
	b.count = 0
}

var (
	orderPool    *OrderPool
	tradePool    *TradePool
	orderCounter int64
	tradeCounter int64
)

func init() {
	orderPool = NewOrderPool()
	tradePool = NewTradePool()
}

func GetOrderPool() *OrderPool {
	return orderPool
}

func GetTradePool() *TradePool {
	return tradePool
}

func NextOrderID() types.OrderID {
	return types.OrderID(atomic.AddInt64(&orderCounter, 1))
}

func NextTradeID() types.OrderID {
	return types.OrderID(atomic.AddInt64(&tradeCounter, 1))
}
