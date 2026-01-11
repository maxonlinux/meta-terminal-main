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
	index     int
}

var nodePool = sync.Pool{
	New: func() interface{} { return &TriggerNode{} },
}

type BuyHeap []*TriggerNode

func (h BuyHeap) Len() int { return len(h) }

func (h BuyHeap) Less(i, j int) bool {
	if h[i].price == h[j].price {
		return h[i].timestamp < h[j].timestamp
	}
	return h[i].price > h[j].price
}

func (h BuyHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *BuyHeap) Push(x interface{}) {
	node := x.(*TriggerNode)
	node.index = len(*h)
	*h = append(*h, node)
}

func (h *BuyHeap) Pop() interface{} {
	old := *h
	n := len(old)
	node := old[n-1]
	node.index = -1
	*h = old[0 : n-1]
	return node
}

func (h *BuyHeap) Peek() *TriggerNode {
	if len(*h) == 0 {
		return nil
	}
	return (*h)[0]
}

type SellHeap []*TriggerNode

func (h SellHeap) Len() int { return len(h) }

func (h SellHeap) Less(i, j int) bool {
	if h[i].price == h[j].price {
		return h[i].timestamp < h[j].timestamp
	}
	return h[i].price < h[j].price
}

func (h SellHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *SellHeap) Push(x interface{}) {
	node := x.(*TriggerNode)
	node.index = len(*h)
	*h = append(*h, node)
}

func (h *SellHeap) Pop() interface{} {
	old := *h
	n := len(old)
	node := old[n-1]
	node.index = -1
	*h = old[0 : n-1]
	return node
}

func (h *SellHeap) Peek() *TriggerNode {
	if len(*h) == 0 {
		return nil
	}
	return (*h)[0]
}

type Monitor struct {
	mu           sync.Mutex
	buyTriggers  *BuyHeap
	sellTriggers *SellHeap
	orders       map[types.OrderID]*types.Order
	nodes        map[types.OrderID]*TriggerNode
}

func New() *Monitor {
	return NewWithCapacity(0)
}

func NewWithCapacity(capacity int) *Monitor {
	m := &Monitor{
		buyTriggers:  &BuyHeap{},
		sellTriggers: &SellHeap{},
		orders:       make(map[types.OrderID]*types.Order, capacity),
		nodes:        make(map[types.OrderID]*TriggerNode, capacity),
	}
	heap.Init(m.buyTriggers)
	heap.Init(m.sellTriggers)
	return m
}

func (m *Monitor) Add(order *types.Order) {
	m.mu.Lock()
	defer m.mu.Unlock()

	triggerSide := effectiveTriggerSide(order)
	node := nodePool.Get().(*TriggerNode)
	*node = TriggerNode{
		orderID:   order.ID,
		price:     order.TriggerPrice,
		side:      triggerSide,
		timestamp: order.CreatedAt,
		index:     -1,
	}

	m.orders[order.ID] = order
	m.nodes[order.ID] = node

	if triggerSide == constants.ORDER_SIDE_BUY {
		heap.Push(m.buyTriggers, node)
	} else {
		heap.Push(m.sellTriggers, node)
	}
}

func (m *Monitor) Remove(orderID types.OrderID) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	node := m.nodes[orderID]
	if node == nil {
		return false
	}

	delete(m.nodes, orderID)
	delete(m.orders, orderID)
	if node.side == constants.ORDER_SIDE_BUY {
		heap.Remove(m.buyTriggers, node.index)
	} else {
		heap.Remove(m.sellTriggers, node.index)
	}
	*node = TriggerNode{}
	nodePool.Put(node)
	return true
}

func (m *Monitor) Check(currentPrice types.Price) []*types.Order {
	return m.CheckInto(currentPrice, nil)
}

func (m *Monitor) CheckInto(currentPrice types.Price, out []*types.Order) []*types.Order {
	m.mu.Lock()
	defer m.mu.Unlock()

	triggered := out
	if triggered != nil {
		triggered = triggered[:0]
	}

	for m.buyTriggers.Len() > 0 {
		node := m.buyTriggers.Peek()
		if node == nil || node.price < currentPrice {
			break
		}
		heap.Pop(m.buyTriggers)
		if order := m.orders[node.orderID]; order != nil {
			triggered = append(triggered, order)
		}
		delete(m.orders, node.orderID)
		delete(m.nodes, node.orderID)
		*node = TriggerNode{}
		nodePool.Put(node)
	}

	for m.sellTriggers.Len() > 0 {
		node := m.sellTriggers.Peek()
		if node == nil || node.price > currentPrice {
			break
		}
		heap.Pop(m.sellTriggers)
		if order := m.orders[node.orderID]; order != nil {
			triggered = append(triggered, order)
		}
		delete(m.orders, node.orderID)
		delete(m.nodes, node.orderID)
		*node = TriggerNode{}
		nodePool.Put(node)
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

func effectiveTriggerSide(order *types.Order) int8 {
	if !order.CloseOnTrigger {
		return order.Side
	}
	switch order.StopOrderType {
	case constants.STOP_ORDER_TYPE_STOP, constants.STOP_ORDER_TYPE_STOP_LOSS:
		return flipSide(order.Side)
	default:
		return order.Side
	}
}

func flipSide(side int8) int8 {
	if side == constants.ORDER_SIDE_BUY {
		return constants.ORDER_SIDE_SELL
	}
	return constants.ORDER_SIDE_BUY
}
