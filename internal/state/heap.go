package state

import (
	"slices"

	"github.com/anomalyco/meta-terminal-go/internal/types"
)

type OrderLookup interface {
	Get(orderID types.OrderID) *types.Order
}

type Heap struct {
	Data        []types.OrderID
	PriceField  func(*types.Order) types.Price
	OrderLookup OrderLookup
	Max         bool
}

func NewHeap(max bool, priceField func(*types.Order) types.Price, orderLookup OrderLookup) *Heap {
	return &Heap{
		Data:        make([]types.OrderID, 0),
		PriceField:  priceField,
		OrderLookup: orderLookup,
		Max:         max,
	}
}

func (h *Heap) Push(id types.OrderID) {
	h.Data = append(h.Data, id)
	h.siftUp(len(h.Data) - 1)
}

func (h *Heap) Pop() types.OrderID {
	if len(h.Data) == 0 {
		return 0
	}
	result := h.Data[0]
	h.Data[0] = h.Data[len(h.Data)-1]
	h.Data = h.Data[:len(h.Data)-1]
	if len(h.Data) > 0 {
		h.siftDown(0)
	}
	return result
}

func (h *Heap) Peek() types.OrderID {
	if len(h.Data) == 0 {
		return 0
	}
	return h.Data[0]
}

func (h *Heap) Len() int {
	return len(h.Data)
}

func (h *Heap) siftUp(idx int) {
	for idx > 0 {
		parent := (idx - 1) / 2
		if h.less(parent, idx) {
			break
		}
		h.Data[parent], h.Data[idx] = h.Data[idx], h.Data[parent]
		idx = parent
	}
}

func (h *Heap) siftDown(idx int) {
	n := len(h.Data)
	for {
		left := 2*idx + 1
		right := 2*idx + 2
		smallest := idx
		if left < n && h.less(left, smallest) {
			smallest = left
		}
		if right < n && h.less(right, smallest) {
			smallest = right
		}
		if smallest == idx {
			break
		}
		h.Data[idx], h.Data[smallest] = h.Data[smallest], h.Data[idx]
		idx = smallest
	}
}

func (h *Heap) less(i, j int) bool {
	if i < 0 || i >= len(h.Data) || j < 0 || j >= len(h.Data) {
		return false
	}
	orderI := h.OrderLookup.Get(h.Data[i])
	orderJ := h.OrderLookup.Get(h.Data[j])
	if orderI == nil || orderJ == nil {
		return false
	}
	priceI := h.PriceField(orderI)
	priceJ := h.PriceField(orderJ)
	if h.Max {
		return priceI > priceJ
	}
	return priceI < priceJ
}

func (h *Heap) Remove(orderID types.OrderID) {
	for i, oid := range h.Data {
		if oid == orderID {
			h.Data = slices.Delete(h.Data, i, i+1)
			h.siftUp(i)
			h.siftDown(i)
			return
		}
	}
}
