package oms

import (
	"container/heap"

	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

// ConditionalIndex — структура для хранения условных ордеров
// BUY: активируется когда price ≤ trigger (min-heap)
// SELL: активируется когда price ≥ trigger (max-heap)
type ConditionalIndex struct {
	BuyTriggers  map[string]*TriggerHeap // symbol -> BUY triggers (min-heap by trigger price)
	SellTriggers map[string]*TriggerHeap // symbol -> SELL triggers (max-heap by trigger price)
}

// TriggerHeap — heap для условных ордеров одного символа и стороны
type TriggerHeap struct{ Items []*types.Order }

func (h TriggerHeap) Len() int            { return len(h.Items) }
func (h TriggerHeap) Less(i, j int) bool  { return h.Items[i].TriggerPrice < h.Items[j].TriggerPrice }
func (h TriggerHeap) Swap(i, j int)       { h.Items[i], h.Items[j] = h.Items[j], h.Items[i] }
func (h *TriggerHeap) Push(x interface{}) { h.Items = append(h.Items, x.(*types.Order)) }
func (h *TriggerHeap) Pop() interface{} {
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

func newConditionalIndex() *ConditionalIndex {
	return &ConditionalIndex{
		BuyTriggers:  make(map[string]*TriggerHeap),
		SellTriggers: make(map[string]*TriggerHeap),
	}
}

func (i *ConditionalIndex) add(o *types.Order) {
	if !o.IsConditional {
		return
	}

	var h *TriggerHeap
	switch o.Side {
	case constants.ORDER_SIDE_BUY:
		h = i.BuyTriggers[o.Symbol]
		if h == nil {
			h = &TriggerHeap{}
			i.BuyTriggers[o.Symbol] = h
		}
	case constants.ORDER_SIDE_SELL:
		h = i.SellTriggers[o.Symbol]
		if h == nil {
			h = &TriggerHeap{}
			i.SellTriggers[o.Symbol] = h
		}
	default:
		return
	}
	heap.Push(h, o)
}

func (i *ConditionalIndex) remove(o *types.Order) {
	if !o.IsConditional {
		return
	}

	// TODO: lazy deletion
	o.Status = constants.ORDER_STATUS_DEACTIVATED
}

func (i *ConditionalIndex) getHeap(symbol string, side int8) *TriggerHeap {
	switch side {
	case constants.ORDER_SIDE_BUY:
		return i.BuyTriggers[symbol]
	case constants.ORDER_SIDE_SELL:
		return i.SellTriggers[symbol]
	default:
		return nil
	}
}
