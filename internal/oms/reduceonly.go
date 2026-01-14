package oms

import (
	"container/heap"

	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

type ReduceOnlyManager struct {
	buyHeaps  map[string]*orderHeap
	sellHeaps map[string]*orderHeap
}

type orderHeap struct{ items []*types.Order }

func (h orderHeap) Len() int            { return len(h.items) }
func (h orderHeap) Less(i, j int) bool  { return h.items[i].Price < h.items[j].Price }
func (h orderHeap) Swap(i, j int)       { h.items[i], h.items[j] = h.items[j], h.items[i] }
func (h *orderHeap) Push(x interface{}) { h.items = append(h.items, x.(*types.Order)) }
func (h *orderHeap) Pop() interface{} {
	n := len(h.items)
	x := h.items[n-1]
	h.items = h.items[:n-1]
	return x
}

func NewReduceOnlyManager() *ReduceOnlyManager {
	return &ReduceOnlyManager{
		buyHeaps:  make(map[string]*orderHeap),
		sellHeaps: make(map[string]*orderHeap),
	}
}

func (m *ReduceOnlyManager) Add(o *types.Order) {
	if !o.ReduceOnly {
		return
	}

	var h *orderHeap
	switch o.Side {
	case constants.ORDER_SIDE_BUY:
		h = m.buyHeaps[o.Symbol]
		if h == nil {
			h = &orderHeap{}
			m.buyHeaps[o.Symbol] = h
		}
	case constants.ORDER_SIDE_SELL:
		h = m.sellHeaps[o.Symbol]
		if h == nil {
			h = &orderHeap{}
			m.sellHeaps[o.Symbol] = h
		}
	default:
		return
	}
	heap.Push(h, o)
}

func (m *ReduceOnlyManager) Trim(symbol string, positionSize types.Quantity) {
	// Size > 0 → LONG → trim SELL
	// Size < 0 → SHORT → trim BUY
	var h *orderHeap
	if positionSize > 0 {
		h = m.sellHeaps[symbol]
	} else {
		h = m.buyHeaps[symbol]
	}

	if h.Len() == 0 {
		return
	}

	var total types.Quantity
	for _, o := range h.items {
		total += o.Quantity - o.Filled
	}
	if total <= positionSize {
		return
	}

	excess := total - positionSize
	for h.Len() > 0 && excess > 0 {
		o := heap.Pop(h).(*types.Order)
		remaining := o.Quantity - o.Filled

		if remaining <= excess {
			o.Status = constants.ORDER_STATUS_CANCELED
			o.ClosedAt = types.NowNano()
			excess -= remaining
		} else {
			o.Quantity -= excess
			excess = 0
		}
	}
}
