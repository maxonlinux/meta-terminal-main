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
	timestamp uint64
}

var nodePool = sync.Pool{
	New: func() interface{} { return &TriggerNode{} },
}

type buyHeap []*TriggerNode

func (h buyHeap) Len() int { return len(h) }
func (h buyHeap) Less(i, j int) bool {
	if h[i].price == h[j].price {
		return h[i].timestamp < h[j].timestamp
	}
	return h[i].price < h[j].price
}
func (h buyHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *buyHeap) Push(x interface{}) { *h = append(*h, x.(*TriggerNode)) }
func (h *buyHeap) Pop() interface{} {
	old := *h
	n := len(old)
	node := old[n-1]
	*h = old[:n-1]
	return node
}

type sellHeap []*TriggerNode

func (h sellHeap) Len() int { return len(h) }
func (h sellHeap) Less(i, j int) bool {
	if h[i].price == h[j].price {
		return h[i].timestamp < h[j].timestamp
	}
	return h[i].price > h[j].price
}
func (h sellHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *sellHeap) Push(x interface{}) { *h = append(*h, x.(*TriggerNode)) }
func (h *sellHeap) Pop() interface{} {
	old := *h
	n := len(old)
	node := old[n-1]
	*h = old[:n-1]
	return node
}

type Monitor struct {
	mu sync.Mutex

	buyHeap  buyHeap
	sellHeap sellHeap

	orders map[types.OrderID]*types.Order
}

func New() *Monitor {
	return NewWithCapacity(0)
}

func NewWithCapacity(capacity int) *Monitor {
	return &Monitor{
		orders: make(map[types.OrderID]*types.Order, capacity),
	}
}

func (m *Monitor) Add(order *types.Order) {
	m.mu.Lock()
	defer m.mu.Unlock()

	node := nodePool.Get().(*TriggerNode)
	*node = TriggerNode{
		orderID:   order.ID,
		price:     order.TriggerPrice,
		timestamp: order.CreatedAt,
	}

	m.orders[order.ID] = order

	if order.Side == constants.ORDER_SIDE_BUY {
		heap.Push(&m.buyHeap, node)
	} else {
		heap.Push(&m.sellHeap, node)
	}
}

func (m *Monitor) Remove(orderID types.OrderID) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	order, ok := m.orders[orderID]
	if !ok {
		return false
	}

	if order.Side == constants.ORDER_SIDE_BUY {
		m.removeBuy(orderID)
	} else {
		m.removeSell(orderID)
	}

	delete(m.orders, orderID)
	return true
}

func (m *Monitor) removeBuy(orderID types.OrderID) {
	for i, node := range m.buyHeap {
		if node.orderID == orderID {
			last := len(m.buyHeap) - 1
			if i != last {
				m.buyHeap[i] = m.buyHeap[last]
			}
			m.buyHeap = m.buyHeap[:last]
			heap.Init(&m.buyHeap)
			break
		}
	}
}

func (m *Monitor) removeSell(orderID types.OrderID) {
	for i, node := range m.sellHeap {
		if node.orderID == orderID {
			last := len(m.sellHeap) - 1
			if i != last {
				m.sellHeap[i] = m.sellHeap[last]
			}
			m.sellHeap = m.sellHeap[:last]
			heap.Init(&m.sellHeap)
			break
		}
	}
}

func (m *Monitor) Check(currentPrice types.Price) []*types.Order {
	return m.CheckInto(currentPrice, nil)
}

func (m *Monitor) CheckInto(currentPrice types.Price, out []*types.Order) []*types.Order {
	m.mu.Lock()
	defer m.mu.Unlock()

	if out != nil {
		out = out[:0]
	} else {
		out = make([]*types.Order, 0, 32)
	}

	for m.buyHeap.Len() > 0 && m.buyHeap[0].price <= currentPrice {
		node := heap.Pop(&m.buyHeap).(*TriggerNode)
		if order := m.orders[node.orderID]; order != nil {
			out = append(out, order)
			delete(m.orders, node.orderID)
		}
		*node = TriggerNode{}
		nodePool.Put(node)
	}

	for m.sellHeap.Len() > 0 && m.sellHeap[0].price >= currentPrice {
		node := heap.Pop(&m.sellHeap).(*TriggerNode)
		if order := m.orders[node.orderID]; order != nil {
			out = append(out, order)
			delete(m.orders, node.orderID)
		}
		*node = TriggerNode{}
		nodePool.Put(node)
	}

	return out
}

func (m *Monitor) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.orders)
}
