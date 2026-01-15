package oms

import (
	"container/heap"

	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

type ReduceOnlyIndex struct {
	buyHeaps  map[string]*orderHeap
	sellHeaps map[string]*orderHeap
	exposure  map[string]map[types.UserID]types.Quantity
	deleted   map[*types.OrderID]bool
}

type orderHeap struct{ items []*types.Order }

func (h orderHeap) Len() int { return len(h.items) }

func (h orderHeap) Less(i, j int) bool {
	return math.Cmp(h.items[i].Price, h.items[j].Price) < 0
}

func (h orderHeap) Swap(i, j int) {
	h.items[i], h.items[j] = h.items[j], h.items[i]
}

func (h *orderHeap) Push(x any) {
	h.items = append(h.items, x.(*types.Order))
}

func (h *orderHeap) Pop() any {
	n := len(h.items)
	x := h.items[n-1]
	h.items = h.items[:n-1]
	return x
}

func (h *orderHeap) Peek() *types.Order {
	if h.Len() == 0 {
		return nil
	}
	return h.items[0]
}

func NewReduceOnlyManager() *ReduceOnlyIndex {
	return &ReduceOnlyIndex{
		buyHeaps:  make(map[string]*orderHeap),
		sellHeaps: make(map[string]*orderHeap),
		exposure:  make(map[string]map[types.UserID]types.Quantity),
		deleted:   make(map[*types.OrderID]bool),
	}
}

func (r *ReduceOnlyIndex) Add(o *types.Order) {
	if !o.ReduceOnly {
		return
	}

	if r.exposure[o.Symbol] == nil {
		r.exposure[o.Symbol] = make(map[types.UserID]types.Quantity)
	}

	remaining := math.Sub(o.Quantity, o.Filled)
	r.exposure[o.Symbol][o.UserID] = math.Add(r.exposure[o.Symbol][o.UserID], remaining)

	var h *orderHeap
	switch o.Side {
	case constants.ORDER_SIDE_BUY:
		h = r.buyHeaps[o.Symbol]
		if h == nil {
			h = &orderHeap{}
			r.buyHeaps[o.Symbol] = h
		}
	case constants.ORDER_SIDE_SELL:
		h = r.sellHeaps[o.Symbol]
		if h == nil {
			h = &orderHeap{}
			r.sellHeaps[o.Symbol] = h
		}
	default:
		return
	}
	heap.Push(h, o)
}

func (r *ReduceOnlyIndex) Remove(o *types.Order) {
	remaining := math.Sub(o.Quantity, o.Filled)
	r.exposure[o.Symbol][o.UserID] = math.Sub(r.exposure[o.Symbol][o.UserID], remaining)
	r.deleted[&o.ID] = true
}
