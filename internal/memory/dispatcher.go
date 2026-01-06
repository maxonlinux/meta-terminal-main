package memory

import (
	"sync"

	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

type Dispatcher struct {
	state  *state.State
	queues map[types.UserID]*UserQueue
	mu     sync.RWMutex
}

func NewDispatcher(s *state.State) *Dispatcher {
	return &Dispatcher{
		state:  s,
		queues: make(map[types.UserID]*UserQueue),
	}
}

func (d *Dispatcher) GetQueue(userID types.UserID) *UserQueue {
	d.mu.Lock()
	defer d.mu.Unlock()
	q, ok := d.queues[userID]
	if !ok {
		q = NewUserQueue()
		d.queues[userID] = q
	}
	return q
}

func (d *Dispatcher) RemoveQueue(userID types.UserID) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if q, ok := d.queues[userID]; ok {
		q.Close()
		delete(d.queues, userID)
	}
}

func (d *Dispatcher) Dispatch(userID types.UserID, order *types.Order) *types.OrderResult {
	q := d.GetQueue(userID)
	q.Push(order.ID)

	return d.processQueue(q, userID)
}

func (d *Dispatcher) processQueue(q *UserQueue, userID types.UserID) *types.OrderResult {
	orderID := q.Pop()
	if orderID == 0 {
		return nil
	}

	var order *types.Order
	for _, ss := range d.state.Symbols {
		if o, ok := ss.OrderMap[orderID]; ok {
			order = o
			break
		}
	}
	if order == nil {
		return nil
	}

	return &types.OrderResult{
		Order: order,
	}
}
