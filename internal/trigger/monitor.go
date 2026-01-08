package trigger

import (
	"container/heap"
	"sync"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

type item struct {
	orderID types.OrderID
	price   types.Price
	side    int8
	index   int
}

type buyMaxHeap []*item

func (h buyMaxHeap) Len() int { return len(h) }
func (h buyMaxHeap) Less(i, j int) bool {
	if h[i].price == h[j].price {
		return h[i].orderID < h[j].orderID
	}
	return h[i].price > h[j].price
}
func (h buyMaxHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}
func (h *buyMaxHeap) Push(x any) {
	it := x.(*item)
	it.index = len(*h)
	*h = append(*h, it)
}
func (h *buyMaxHeap) Pop() any {
	old := *h
	n := len(old)
	it := old[n-1]
	old[n-1] = nil
	it.index = -1
	*h = old[:n-1]
	return it
}

type sellMinHeap []*item

func (h sellMinHeap) Len() int { return len(h) }
func (h sellMinHeap) Less(i, j int) bool {
	if h[i].price == h[j].price {
		return h[i].orderID < h[j].orderID
	}
	return h[i].price < h[j].price
}
func (h sellMinHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}
func (h *sellMinHeap) Push(x any) {
	it := x.(*item)
	it.index = len(*h)
	*h = append(*h, it)
}
func (h *sellMinHeap) Pop() any {
	old := *h
	n := len(old)
	it := old[n-1]
	old[n-1] = nil
	it.index = -1
	*h = old[:n-1]
	return it
}

type Monitor struct {
	mu sync.Mutex

	buy  buyMaxHeap
	sell sellMinHeap

	index map[types.OrderID]*item
}

func NewMonitor() *Monitor {
	m := &Monitor{index: make(map[types.OrderID]*item)}
	heap.Init(&m.buy)
	heap.Init(&m.sell)
	return m
}

func (m *Monitor) Add(orderID types.OrderID, side int8, triggerPrice types.Price) {
	if triggerPrice <= 0 {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.index[orderID] != nil {
		return
	}

	it := &item{orderID: orderID, side: side, price: triggerPrice}
	m.index[orderID] = it

	if side == constants.ORDER_SIDE_BUY {
		heap.Push(&m.buy, it)
		return
	}
	heap.Push(&m.sell, it)
}

func (m *Monitor) Remove(orderID types.OrderID) {
	m.mu.Lock()
	defer m.mu.Unlock()

	it := m.index[orderID]
	if it == nil {
		return
	}
	delete(m.index, orderID)

	if it.side == constants.ORDER_SIDE_BUY {
		heap.Remove(&m.buy, it.index)
		return
	}
	heap.Remove(&m.sell, it.index)
}

func (m *Monitor) Check(currentPrice types.Price) []types.OrderID {
	m.mu.Lock()
	defer m.mu.Unlock()

	var triggered []types.OrderID

	for m.buy.Len() > 0 {
		it := m.buy[0]
		if currentPrice > it.price {
			break
		}
		heap.Pop(&m.buy)
		delete(m.index, it.orderID)
		triggered = append(triggered, it.orderID)
	}

	for m.sell.Len() > 0 {
		it := m.sell[0]
		if currentPrice < it.price {
			break
		}
		heap.Pop(&m.sell)
		delete(m.index, it.orderID)
		triggered = append(triggered, it.orderID)
	}

	return triggered
}
