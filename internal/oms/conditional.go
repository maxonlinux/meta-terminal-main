package oms

import (
	"container/heap"

	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

type ConditionalIndex struct {
	buyTriggers  map[string]*TriggerHeap
	sellTriggers map[string]*TriggerHeap
	deleted      map[*types.OrderID]bool
}

type TriggerHeap struct {
	Items []*types.Order
}

func (h TriggerHeap) Len() int { return len(h.Items) }

func (h TriggerHeap) Less(i, j int) bool {
	return math.Cmp(h.Items[i].TriggerPrice, h.Items[j].TriggerPrice) < 0
}

func (h TriggerHeap) Swap(i, j int) {
	h.Items[i], h.Items[j] = h.Items[j], h.Items[i]
}

func (h *TriggerHeap) Push(x any) {
	h.Items = append(h.Items, x.(*types.Order))
}

func (h *TriggerHeap) Pop() any {
	n := len(h.Items)
	x := h.Items[n-1]
	h.Items = h.Items[:n-1]
	return x
}

func (h *TriggerHeap) Peek() *types.Order {
	if h.Len() == 0 {
		return nil
	}
	return h.Items[0]
}

func NewConditionalIndex() *ConditionalIndex {
	return &ConditionalIndex{
		buyTriggers:  make(map[string]*TriggerHeap),
		sellTriggers: make(map[string]*TriggerHeap),
		deleted:      make(map[*types.OrderID]bool),
	}
}

func (c *ConditionalIndex) Add(o *types.Order) {
	if o.TriggerPrice.IsZero() {
		return
	}

	var h *TriggerHeap
	switch o.Side {
	case constants.ORDER_SIDE_BUY:
		h = c.buyTriggers[o.Symbol]
		if h == nil {
			h = &TriggerHeap{}
			c.buyTriggers[o.Symbol] = h
		}
	case constants.ORDER_SIDE_SELL:
		h = c.sellTriggers[o.Symbol]
		if h == nil {
			h = &TriggerHeap{}
			c.sellTriggers[o.Symbol] = h
		}
	default:
		return
	}
	heap.Push(h, o)
}

func (m *ConditionalIndex) Remove(o *types.Order) {
	m.deleted[&o.ID] = true
}
