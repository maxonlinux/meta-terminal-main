package trigger

import (
	"container/heap"
	"sync"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/domain"
)

type item struct {
	orderID domain.OrderID
	price   domain.Price
	side    int8
	index   int
}

type buyMinHeap []*item

func (h buyMinHeap) Len() int { return len(h) }
func (h buyMinHeap) Less(i, j int) bool {
	if h[i].price == h[j].price {
		return h[i].orderID < h[j].orderID
	}
	return h[i].price < h[j].price
}
func (h buyMinHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}
func (h *buyMinHeap) Push(x any) {
	it := x.(*item)
	it.index = len(*h)
	*h = append(*h, it)
}
func (h *buyMinHeap) Pop() any {
	old := *h
	n := len(old)
	it := old[n-1]
	old[n-1] = nil
	it.index = -1
	*h = old[:n-1]
	return it
}

type sellMaxHeap []*item

func (h sellMaxHeap) Len() int { return len(h) }
func (h sellMaxHeap) Less(i, j int) bool {
	if h[i].price == h[j].price {
		return h[i].orderID < h[j].orderID
	}
	return h[i].price > h[j].price
}
func (h sellMaxHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}
func (h *sellMaxHeap) Push(x any) {
	it := x.(*item)
	it.index = len(*h)
	*h = append(*h, it)
}
func (h *sellMaxHeap) Pop() any {
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

	buy  buyMinHeap
	sell sellMaxHeap

	index map[domain.OrderID]*item
}

func NewMonitor() *Monitor {
	m := &Monitor{index: make(map[domain.OrderID]*item)}
	heap.Init(&m.buy)
	heap.Init(&m.sell)
	return m
}

func (m *Monitor) Add(orderID domain.OrderID, side int8, triggerPrice domain.Price) {
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

func (m *Monitor) Remove(orderID domain.OrderID) {
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

func (m *Monitor) Check(currentPrice domain.Price) []domain.OrderID {
	m.mu.Lock()
	defer m.mu.Unlock()

	var triggered []domain.OrderID

	// For performance: BUY triggers when currentPrice >= trigger (MIN heap).
	for m.buy.Len() > 0 {
		it := m.buy[0]
		if currentPrice < it.price {
			break
		}
		heap.Pop(&m.buy)
		delete(m.index, it.orderID)
		triggered = append(triggered, it.orderID)
	}

	// SELL triggers when currentPrice <= trigger (MAX heap).
	for m.sell.Len() > 0 {
		it := m.sell[0]
		if currentPrice > it.price {
			break
		}
		heap.Pop(&m.sell)
		delete(m.index, it.orderID)
		triggered = append(triggered, it.orderID)
	}

	return triggered
}
