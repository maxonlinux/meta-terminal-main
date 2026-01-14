package oms

import (
	"container/heap"

	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

// ReduceOnlyIndex — структура для хранения reduce-only ордеров, отсортированных по цене
// Используется для Bybit-подобного trimming: от дальних к ближним
type ReduceOnlyIndex struct {
	BuyHeaps  map[string]*OrderHeap                      // symbol -> BUY RO (min-heap by price)
	SellHeaps map[string]*OrderHeap                      // symbol -> SELL RO (max-heap by price)
	Exposure  map[string]map[types.UserID]types.Quantity // symbol -> user -> total RO qty
}

// OrderHeap — heap для RO ордеров одного символа и стороны
type OrderHeap struct{ Items []*types.Order }

func (h OrderHeap) Len() int            { return len(h.Items) }
func (h OrderHeap) Less(i, j int) bool  { return h.Items[i].Price < h.Items[j].Price }
func (h OrderHeap) Swap(i, j int)       { h.Items[i], h.Items[j] = h.Items[j], h.Items[i] }
func (h *OrderHeap) Push(x interface{}) { h.Items = append(h.Items, x.(*types.Order)) }
func (h *OrderHeap) Pop() interface{} {
	n := len(h.Items)
	x := h.Items[n-1]
	h.Items = h.Items[:n-1]
	return x
}
func (h *OrderHeap) Peek() *types.Order {
	if h.Len() == 0 {
		return nil
	}
	return h.Items[0]
}

func NewReduceOnlyIndex() *ReduceOnlyIndex {
	return &ReduceOnlyIndex{
		BuyHeaps:  make(map[string]*OrderHeap),
		SellHeaps: make(map[string]*OrderHeap),
		Exposure:  make(map[string]map[types.UserID]types.Quantity),
	}
}

func (i *ReduceOnlyIndex) Add(o *types.Order) {
	if !o.ReduceOnly {
		return
	}

	if i.Exposure[o.Symbol] == nil {
		i.Exposure[o.Symbol] = make(map[types.UserID]types.Quantity)
	}
	i.Exposure[o.Symbol][o.UserID] += o.Quantity - o.Filled

	var h *OrderHeap
	switch o.Side {
	case constants.ORDER_SIDE_BUY:
		h = i.BuyHeaps[o.Symbol]
		if h == nil {
			h = &OrderHeap{}
			i.BuyHeaps[o.Symbol] = h
		}
	case constants.ORDER_SIDE_SELL:
		h = i.SellHeaps[o.Symbol]
		if h == nil {
			h = &OrderHeap{}
			i.SellHeaps[o.Symbol] = h
		}
	default:
		return
	}
	heap.Push(h, o)
}

func (i *ReduceOnlyIndex) Remove(orderID types.OrderID) {
	// TODO: lazy deletion или полное перестроение heap
	// Пока O(n) поиск, затем heap.Fix
}

func (i *ReduceOnlyIndex) GetHeap(symbol string, positionSize types.Quantity) *OrderHeap {
	if positionSize > 0 {
		return i.SellHeaps[symbol]
	}
	return i.BuyHeaps[symbol]
}
