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
	shards  [constants.OMS_SHARD_COUNT]*condShard
	deleted map[types.OrderID]bool
}

type triggerHeap struct {
	items []*types.Order
	isBuy bool
}

func (h triggerHeap) Len() int { return len(h.items) }

func (h triggerHeap) Less(i, j int) bool {
	if h.isBuy {
		return math.Cmp(h.items[i].TriggerPrice, h.items[j].TriggerPrice) > 0
	}
	return math.Cmp(h.items[i].TriggerPrice, h.items[j].TriggerPrice) < 0
}

func (h triggerHeap) Swap(i, j int) { h.items[i], h.items[j] = h.items[j], h.items[i] }

func (h *triggerHeap) Push(x any) { h.items = append(h.items, x.(*types.Order)) }

func (h *triggerHeap) Pop() any {
	n := len(h.items)
	item := h.items[n-1]
	h.items = h.items[:n-1]
	return item
}

func (h *triggerHeap) Peek() *types.Order {
	for h.Len() > 0 {
		order := h.items[0]
		if order.Status != constants.ORDER_STATUS_UNTRIGGERED {
			heap.Pop(h)
			continue
		}
		return order
	}
	return nil
}

func NewConditionalIndex() *ConditionalIndex {
	c := &ConditionalIndex{
		deleted: make(map[types.OrderID]bool),
	}
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

	delete(c.deleted, o.ID)

	h := c.getHeap(o.Symbol, o.Side == constants.ORDER_SIDE_BUY)
	heap.Push(h, o)
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
		h = &triggerHeap{isBuy: isBuy}
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

func (c *ConditionalIndex) CheckTriggers(symbol string, currentPrice types.Price, callback func(*types.Order)) {
	shardIdx := ShardIndex(symbol)
	shard := c.shards[shardIdx]

	if h := shard.buyTriggers[symbol]; h != nil {
		trigger := func(o *types.Order) {
			o.Status = constants.ORDER_STATUS_TRIGGERED
			o.UpdatedAt = utils.NowNano()
			callback(o)
			heap.Pop(h)
		}
		triggerHeapForPrice(h, currentPrice, trigger, 1)
	}

	if h := shard.sellTriggers[symbol]; h != nil {
		trigger := func(o *types.Order) {
			o.Status = constants.ORDER_STATUS_TRIGGERED
			o.UpdatedAt = utils.NowNano()
			callback(o)
			heap.Pop(h)
		}
		triggerHeapForPrice(h, currentPrice, trigger, -1)
	}
}

func (c *ConditionalIndex) Remove(o *types.Order) {
	if o.TriggerPrice.IsZero() {
		return
	}
	c.deleted[o.ID] = true
}
