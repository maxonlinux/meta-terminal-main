package oms

import (
	"container/heap"

	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/maxonlinux/meta-terminal-go/pkg/utils"
)

type condShard struct {
	buyTriggers  map[string]*triggerHeap
	sellTriggers map[string]*triggerHeap
}

type ConditionalIndex struct {
	shards [constants.OMS_SHARD_COUNT]*condShard
}

type triggerHeap struct {
	items []*triggerItem
	index map[types.OrderID]*triggerItem
	isBuy bool
}

type triggerItem struct {
	order *types.Order
	idx   int
}

func (h triggerHeap) Len() int { return len(h.items) }

func (h triggerHeap) Less(i, j int) bool {
	if h.isBuy {
		return math.Cmp(h.items[i].order.TriggerPrice, h.items[j].order.TriggerPrice) > 0
	}
	return math.Cmp(h.items[i].order.TriggerPrice, h.items[j].order.TriggerPrice) < 0
}

func (h triggerHeap) Swap(i, j int) {
	h.items[i], h.items[j] = h.items[j], h.items[i]
	h.items[i].idx = i
	h.items[j].idx = j
}

func (h *triggerHeap) Push(x any) {
	item := x.(*triggerItem)
	item.idx = len(h.items)
	h.items = append(h.items, item)
	h.index[item.order.ID] = item
}

func (h *triggerHeap) Pop() any {
	n := len(h.items)
	item := h.items[n-1]
	item.idx = -1
	h.items = h.items[:n-1]
	delete(h.index, item.order.ID)
	return item
}

func (h *triggerHeap) Peek() *types.Order {
	for h.Len() > 0 {
		order := h.items[0].order
		if order.Status != constants.ORDER_STATUS_UNTRIGGERED {
			heap.Pop(h)
			continue
		}
		return order
	}
	return nil
}

func NewConditionalIndex() *ConditionalIndex {
	c := &ConditionalIndex{}
	for i := range c.shards {
		c.shards[i] = &condShard{
			buyTriggers:  make(map[string]*triggerHeap),
			sellTriggers: make(map[string]*triggerHeap),
		}
	}
	return c
}

func (c *ConditionalIndex) Add(o *types.Order) {
	if o.TriggerPrice.IsZero() {
		return
	}
	h := c.getHeap(o.Symbol, triggerIsBuy(o.TriggerDirection, o.Side))
	item := &triggerItem{order: o, idx: -1}
	heap.Push(h, item)
}

func (c *ConditionalIndex) getHeap(symbol string, isBuy bool) *triggerHeap {
	shardIdx := ShardIndex(symbol)
	shard := c.shards[shardIdx]

	triggerMap := shard.buyTriggers
	if !isBuy {
		triggerMap = shard.sellTriggers
	}

	h, ok := triggerMap[symbol]
	if !ok {
		h = &triggerHeap{isBuy: isBuy, index: make(map[types.OrderID]*triggerItem)}
		triggerMap[symbol] = h
	}
	return h
}

func triggerHeapForPrice(h *triggerHeap, currentPrice types.Price, trigger func(*types.Order), cmp int) {
	for h.Len() > 0 {
		o := h.Peek()
		if o == nil {
			return
		}
		if math.Cmp(currentPrice, o.TriggerPrice) == cmp {
			return
		}
		trigger(o)
	}
}

func (c *ConditionalIndex) CheckTriggers(symbol string, currentPrice types.Price) []*types.Order {
	shardIdx := ShardIndex(symbol)
	shard := c.shards[shardIdx]
	var triggered []*types.Order

	if h := shard.buyTriggers[symbol]; h != nil {
		trigger := func(o *types.Order) {
			o.Status = constants.ORDER_STATUS_TRIGGERED
			o.UpdatedAt = utils.NowNano()
			triggered = append(triggered, o)
			heap.Pop(h)
		}
		triggerHeapForPrice(h, currentPrice, trigger, 1)
	}

	if h := shard.sellTriggers[symbol]; h != nil {
		trigger := func(o *types.Order) {
			o.Status = constants.ORDER_STATUS_TRIGGERED
			o.UpdatedAt = utils.NowNano()
			triggered = append(triggered, o)
			heap.Pop(h)
		}
		triggerHeapForPrice(h, currentPrice, trigger, -1)
	}
	return triggered
}

func (c *ConditionalIndex) Remove(o *types.Order) {
	if o.TriggerPrice.IsZero() {
		return
	}
	h := c.getHeap(o.Symbol, triggerIsBuy(o.TriggerDirection, o.Side))
	item := h.index[o.ID]
	if item == nil {
		return
	}
	heap.Remove(h, item.idx)
}

func triggerIsBuy(direction int8, side int8) bool {
	switch direction {
	case constants.TRIGGER_DIRECTION_DOWN:
		return true
	case constants.TRIGGER_DIRECTION_UP:
		return false
	default:
		return side == constants.ORDER_SIDE_BUY
	}
}
