package oms

import (
	"container/heap"

	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/maxonlinux/meta-terminal-go/pkg/utils"
)

// condShard groups trigger heaps per symbol.
type condShard struct {
	buyTriggers  map[string]*triggerHeap
	sellTriggers map[string]*triggerHeap
}

// ConditionalIndex stores trigger heaps sharded by symbol hash.
type ConditionalIndex struct {
	shards [constants.OMS_SHARD_COUNT]*condShard
}

// triggerHeap keeps trigger orders ordered by price direction.
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

// Peek returns the next active trigger, skipping canceled orders.
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

// NewConditionalIndex creates a new ConditionalIndex with pre-allocated shards.
// Each shard has empty trigger maps that are created on-demand.
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

// Add inserts a conditional order into the trigger heap.
func (c *ConditionalIndex) Add(o *types.Order) {
	if o.TriggerPrice.IsZero() {
		return
	}

	shardIdx := ShardIndex(o.Symbol)

	var h *triggerHeap
	switch o.Side {
	case constants.ORDER_SIDE_BUY:
		h = c.getOrCreateHeap(shardIdx, o.Symbol, true)
	case constants.ORDER_SIDE_SELL:
		h = c.getOrCreateHeap(shardIdx, o.Symbol, false)
	default:
		return
	}
	heap.Push(h, o)
}

// getOrCreateHeap retrieves or creates a trigger heap for the given shard, symbol, and side.
func (c *ConditionalIndex) getOrCreateHeap(shardIdx uint8, symbol string, isBuy bool) *triggerHeap {
	triggerMap := c.getTriggerMap(shardIdx, isBuy)
	if h, ok := triggerMap[symbol]; ok {
		return h
	}

	h := &triggerHeap{isBuy: isBuy}
	triggerMap[symbol] = h
	return h
}

func (c *ConditionalIndex) getTriggerMap(shardIdx uint8, isBuy bool) map[string]*triggerHeap {
	shard := c.shards[shardIdx]
	if isBuy {
		return shard.buyTriggers
	}
	return shard.sellTriggers
}

func triggerHeapForPrice(h *triggerHeap, currentPrice types.Price, trigger func(*types.Order, *triggerHeap), cmp int) {
	for h.Len() > 0 {
		o := h.Peek()
		if o == nil {
			return
		}
		if math.Cmp(currentPrice, o.TriggerPrice) == cmp {
			return
		}
		trigger(o, h)
	}
}

// CheckTriggers fires any orders that cross the current price and invokes the callback.
func (c *ConditionalIndex) CheckTriggers(symbol string, currentPrice types.Price, callback func(*types.Order)) {
	shardIdx := ShardIndex(symbol)

	// Trigger now to prevent further matching as conditional.
	trigger := func(o *types.Order, h *triggerHeap) {
		o.Status = constants.ORDER_STATUS_TRIGGERED
		o.UpdatedAt = utils.NowNano()
		callback(o)
		heap.Remove(h, 0)
	}

	if h := c.getTriggerMap(shardIdx, true)[symbol]; h != nil {
		triggerHeapForPrice(h, currentPrice, trigger, 1)
	}

	// SELL triggers: fire when currentPrice >= triggerPrice
	// SELL uses min-heap: lowest trigger first (front of heap)
	if h := c.getTriggerMap(shardIdx, false)[symbol]; h != nil {
		triggerHeapForPrice(h, currentPrice, trigger, -1)
	}
}

// Remove removes a conditional order from its trigger heap.
func (c *ConditionalIndex) Remove(o *types.Order) {
	if !o.IsConditional || o.TriggerPrice.IsZero() {
		return
	}

	shardIdx := ShardIndex(o.Symbol)
	triggerMap := c.getTriggerMap(shardIdx, o.Side == constants.ORDER_SIDE_BUY)
	h := triggerMap[o.Symbol]
	if h == nil {
		return
	}

	for i, item := range h.items {
		if item.ID == o.ID {
			heap.Remove(h, i)
			return
		}
	}
}
