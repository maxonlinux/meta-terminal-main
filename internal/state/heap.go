package state

import "github.com/anomalyco/meta-terminal-go/internal/types"

type Heap struct {
	Data       []types.OrderID
	PriceField func(*types.Order) types.Price
	Max        bool
}

func NewHeap(max bool, priceField func(*types.Order) types.Price) *Heap {
	return &Heap{
		Data:       make([]types.OrderID, 0),
		PriceField: priceField,
		Max:        max,
	}
}

func (h *Heap) Push(orders map[types.OrderID]*types.Order, id types.OrderID) {
	h.Data = append(h.Data, id)
	h.siftUp(orders, len(h.Data)-1)
}

func (h *Heap) Pop(orders map[types.OrderID]*types.Order) types.OrderID {
	if len(h.Data) == 0 {
		return 0
	}
	result := h.Data[0]
	h.Data[0] = h.Data[len(h.Data)-1]
	h.Data = h.Data[:len(h.Data)-1]
	if len(h.Data) > 0 {
		h.siftDown(orders, 0)
	}
	return result
}

func (h *Heap) Peek(orders map[types.OrderID]*types.Order) types.OrderID {
	if len(h.Data) == 0 {
		return 0
	}
	return h.Data[0]
}

func (h *Heap) Len() int {
	return len(h.Data)
}

func (h *Heap) siftUp(orders map[types.OrderID]*types.Order, idx int) {
	for idx > 0 {
		parent := (idx - 1) / 2
		if h.less(orders, parent, idx) {
			break
		}
		h.Data[parent], h.Data[idx] = h.Data[idx], h.Data[parent]
		idx = parent
	}
}

func (h *Heap) siftDown(orders map[types.OrderID]*types.Order, idx int) {
	n := len(h.Data)
	for {
		left := 2*idx + 1
		right := 2*idx + 2
		smallest := idx
		if left < n && h.less(orders, left, smallest) {
			smallest = left
		}
		if right < n && h.less(orders, right, smallest) {
			smallest = right
		}
		if smallest == idx {
			break
		}
		h.Data[idx], h.Data[smallest] = h.Data[smallest], h.Data[idx]
		idx = smallest
	}
}

func (h *Heap) less(orders map[types.OrderID]*types.Order, i, j int) bool {
	if i < 0 || i >= len(h.Data) || j < 0 || j >= len(h.Data) {
		return false
	}
	priceI := h.PriceField(orders[h.Data[i]])
	priceJ := h.PriceField(orders[h.Data[j]])
	if h.Max {
		return priceI > priceJ
	}
	return priceI < priceJ
}
