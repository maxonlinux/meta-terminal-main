package memory

import (
	"sync"
	"sync/atomic"

	"github.com/anomalyco/meta-terminal-go/internal/types"
)

type OrderPool struct {
	stack []*types.Order
	mu    sync.Mutex
	cap   int
}

func NewOrderPool() *OrderPool {
	return &OrderPool{
		stack: make([]*types.Order, 0, 1024),
		cap:   1024,
	}
}

func (p *OrderPool) Get() *types.Order {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.stack) == 0 {
		p.grow()
	}
	o := p.stack[len(p.stack)-1]
	p.stack = p.stack[:len(p.stack)-1]
	return o
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
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.stack) < p.cap {
		p.stack = append(p.stack, o)
	}
}

func (p *OrderPool) grow() {
	for i := 0; i < 256; i++ {
		p.stack = append(p.stack, &types.Order{})
	}
	p.cap += 256
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

type OrderResultPool struct {
	pool sync.Pool
}

func NewOrderResultPool() *OrderResultPool {
	return &OrderResultPool{
		pool: sync.Pool{
			New: func() interface{} {
				return &types.OrderResult{}
			},
		},
	}
}

func (p *OrderResultPool) Get() *types.OrderResult {
	return p.pool.Get().(*types.OrderResult)
}

func (p *OrderResultPool) Put(r *types.OrderResult) {
	if r == nil {
		return
	}
	r.Order = nil
	r.Trades = nil
	r.Filled = 0
	r.Remaining = 0
	r.Status = 0
	p.pool.Put(r)
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
	orderPool       *OrderPool
	tradePool       *TradePool
	orderResultPool *OrderResultPool
	orderCounter    int64
	tradeCounter    int64
)

func init() {
	orderPool = NewOrderPool()
	tradePool = NewTradePool()
	orderResultPool = NewOrderResultPool()
}

func GetOrderPool() *OrderPool {
	return orderPool
}

func GetTradePool() *TradePool {
	return tradePool
}

func GetOrderResultPool() *OrderResultPool {
	return orderResultPool
}

func NextOrderID() types.OrderID {
	return types.OrderID(atomic.AddInt64(&orderCounter, 1))
}

func NextTradeID() types.OrderID {
	return types.OrderID(atomic.AddInt64(&tradeCounter, 1))
}
