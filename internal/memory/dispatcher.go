package memory

import (
	"sync"

	"github.com/anomalyco/meta-terminal-go/internal/orderbook"
	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

type Dispatcher struct {
	queues map[types.UserID]*UserQueue
	mu     sync.RWMutex
	state  *state.State
	pool   *OrderPool
}

func NewDispatcher(s *state.State) *Dispatcher {
	return &Dispatcher{
		queues: make(map[types.UserID]*UserQueue),
		state:  s,
		pool:   GetOrderPool(),
	}
}

func (d *Dispatcher) GetQueue(userID types.UserID) *UserQueue {
	d.mu.RLock()
	q, ok := d.queues[userID]
	d.mu.RUnlock()

	if ok {
		return q
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	if q, ok = d.queues[userID]; ok {
		return q
	}

	q = NewUserQueue(userID)
	d.queues[userID] = q
	return q
}

func (d *Dispatcher) RemoveQueue(userID types.UserID) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.queues, userID)
}

func (d *Dispatcher) Dispatch(order *types.Order) *UserQueueResult {
	q := d.GetQueue(order.UserID)

	q.Push(order)

	go d.processQueue(q)

	return q.GetResult()
}

func (d *Dispatcher) processQueue(q *UserQueue) {
	defer func() {
		if r := recover(); r != nil {
			q.ResultChan() <- &UserQueueResult{Error: r.(error)}
		}
		d.RemoveQueue(q.userID)
	}()

	tradeBuffer := NewTradeBuffer(GetTradePool(), 64)
	totalFilled := types.Quantity(0)
	var lastStatus int8

	for elem := q.orders.Front(); elem != nil && !q.IsClosed(); elem = elem.Next() {
		ord := elem.Value.(*types.Order)

		// Создаём OrderBook для правильного символа
		category := d.state.GetSymbolState(ord.Symbol).Category
		ob := orderbook.New(ord.Symbol, category, d.state)

		trades, _, err := ob.PlaceOrder(ord)
		if err != nil {
			q.ResultChan() <- &UserQueueResult{Error: err}
			return
		}

		// NESTED LOOP!!!!
		for _, trade := range trades {
			tradeBuffer.Add(trade)
			totalFilled += trade.Quantity
		}

		lastStatus = ord.Status

		d.pool.Put(ord)
	}

	q.ResultChan() <- &UserQueueResult{
		Trades: tradeBuffer.Slice(),
		Filled: totalFilled,
		Status: lastStatus,
	}
}
