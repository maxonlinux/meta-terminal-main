package memory

import (
	"sync"

	"github.com/anomalyco/meta-terminal-go/internal/types"
)

type UserQueue struct {
	orders []types.OrderID
	mu     sync.Mutex
	closed bool
}

func NewUserQueue() *UserQueue {
	return &UserQueue{
		orders: make([]types.OrderID, 0),
	}
}

func (q *UserQueue) Push(orderID types.OrderID) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.orders = append(q.orders, orderID)
}

func (q *UserQueue) Pop() types.OrderID {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.orders) == 0 {
		return 0
	}
	orderID := q.orders[0]
	q.orders = q.orders[1:]
	return orderID
}

func (q *UserQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.orders)
}

func (q *UserQueue) IsEmpty() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.orders) == 0
}

func (q *UserQueue) IsClosed() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.closed
}

func (q *UserQueue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.closed = true
}
