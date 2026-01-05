package memory

import (
	"container/list"

	"github.com/anomalyco/meta-terminal-go/internal/types"
)

type UserQueue struct {
	userID types.UserID
	orders *list.List
	closed bool
	result chan *UserQueueResult
}

type UserQueueResult struct {
	Trades    []*types.Trade
	Filled    types.Quantity
	Remaining types.Quantity
	Status    int8
	Error     error
}

func NewUserQueue(userID types.UserID) *UserQueue {
	return &UserQueue{
		userID: userID,
		orders: list.New(),
		result: make(chan *UserQueueResult, 1),
	}
}

func (q *UserQueue) Push(order *types.Order) {
	q.orders.PushBack(order)
}

func (q *UserQueue) Pop() *types.Order {
	if elem := q.orders.Front(); elem != nil {
		q.orders.Remove(elem)
		return elem.Value.(*types.Order)
	}
	return nil
}

func (q *UserQueue) Len() int {
	return q.orders.Len()
}

func (q *UserQueue) Close() {
	q.closed = true
}

func (q *UserQueue) IsClosed() bool {
	return q.closed
}

func (q *UserQueue) ResultChan() chan<- *UserQueueResult {
	return q.result
}

func (q *UserQueue) GetResult() *UserQueueResult {
	return <-q.result
}
