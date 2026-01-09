package triggers

import (
	"container/heap"
	"sync"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

type TriggerNode struct {
	orderID   types.OrderID
	price     types.Price
	side      int8
	timestamp uint64
}

type BuyHeap []TriggerNode

func (h BuyHeap) Len() int { return len(h) }

func (h BuyHeap) Less(i, j int) bool {
	if h[i].price == h[j].price {
		return h[i].timestamp < h[j].timestamp
	}
	return h[i].price < h[j].price
}

func (h BuyHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

func (h *BuyHeap) Push(x interface{}) {
	*h = append(*h, x.(TriggerNode))
}

func (h *BuyHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

func (h *BuyHeap) Peek() TriggerNode {
	if len(*h) == 0 {
		return TriggerNode{}
	}
	return (*h)[0]
}

type SellHeap []TriggerNode

func (h SellHeap) Len() int { return len(h) }

func (h SellHeap) Less(i, j int) bool {
	if h[i].price == h[j].price {
		return h[i].timestamp < h[j].timestamp
	}
	return h[i].price > h[j].price
}

func (h SellHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

func (h *SellHeap) Push(x interface{}) {
	*h = append(*h, x.(TriggerNode))
}

func (h *SellHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

func (h *SellHeap) Peek() TriggerNode {
	if len(*h) == 0 {
		return TriggerNode{}
	}
	return (*h)[0]
}

type Monitor struct {
	mu           sync.Mutex
	buyTriggers  *BuyHeap
	sellTriggers *SellHeap
	orders       map[types.OrderID]*types.Order
	buyPos       map[types.OrderID]int
	sellPos      map[types.OrderID]int
}

func New() *Monitor {
	m := &Monitor{
		buyTriggers:  &BuyHeap{},
		sellTriggers: &SellHeap{},
		orders:       make(map[types.OrderID]*types.Order),
		buyPos:       make(map[types.OrderID]int),
		sellPos:      make(map[types.OrderID]int),
	}
	heap.Init(m.buyTriggers)
	heap.Init(m.sellTriggers)
	return m
}

func (m *Monitor) Add(order *types.Order) {
	m.mu.Lock()
	defer m.mu.Unlock()

	node := TriggerNode{
		orderID:   order.ID,
		price:     order.TriggerPrice,
		side:      order.Side,
		timestamp: order.CreatedAt,
	}

	m.orders[order.ID] = order

	if order.Side == constants.ORDER_SIDE_BUY {
		heap.Push(m.buyTriggers, node)
		m.buyPos[order.ID] = len(*m.buyTriggers) - 1
	} else {
		heap.Push(m.sellTriggers, node)
		m.sellPos[order.ID] = len(*m.sellTriggers) - 1
	}
}

func (m *Monitor) Remove(orderID types.OrderID) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	order := m.orders[orderID]
	if order == nil {
		return false
	}

	if order.Side == constants.ORDER_SIDE_BUY {
		pos, ok := m.buyPos[orderID]
		if !ok {
			return false
		}
		m.removeBuyAt(pos)
		delete(m.buyPos, orderID)
	} else {
		pos, ok := m.sellPos[orderID]
		if !ok {
			return false
		}
		m.removeSellAt(pos)
		delete(m.sellPos, orderID)
	}

	delete(m.orders, orderID)
	return true
}

func (m *Monitor) removeBuyAt(pos int) {
	h := m.buyTriggers
	last := len(*h) - 1

	if pos != last {
		removedOrderID := (*h)[pos].orderID
		(*h)[pos], (*h)[last] = (*h)[last], (*h)[pos]
		heap.Fix(h, pos)
		m.buyPos[(*h)[pos].orderID] = pos
		delete(m.buyPos, removedOrderID)
	}

	heap.Pop(h)
}

func (m *Monitor) removeSellAt(pos int) {
	h := m.sellTriggers
	last := len(*h) - 1

	if pos != last {
		removedOrderID := (*h)[pos].orderID
		(*h)[pos], (*h)[last] = (*h)[last], (*h)[pos]
		heap.Fix(h, pos)
		m.sellPos[(*h)[pos].orderID] = pos
		delete(m.sellPos, removedOrderID)
	}

	heap.Pop(h)
}

func (m *Monitor) Check(currentPrice types.Price) []types.OrderID {
	m.mu.Lock()
	defer m.mu.Unlock()

	var triggered []types.OrderID

	for m.buyTriggers.Len() > 0 {
		node := m.buyTriggers.Peek()
		if node.price > currentPrice {
			break
		}
		heap.Pop(m.buyTriggers)
		delete(m.buyPos, node.orderID)
		triggered = append(triggered, node.orderID)
	}

	for m.sellTriggers.Len() > 0 {
		node := m.sellTriggers.Peek()
		if node.price < currentPrice {
			break
		}
		heap.Pop(m.sellTriggers)
		delete(m.sellPos, node.orderID)
		triggered = append(triggered, node.orderID)
	}

	return triggered
}

func (m *Monitor) GetOrder(orderID types.OrderID) *types.Order {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.orders[orderID]
}

func (m *Monitor) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.orders)
}

func (m *Monitor) BuyCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.buyTriggers.Len()
}

func (m *Monitor) SellCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sellTriggers.Len()
}
