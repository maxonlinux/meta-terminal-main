package oms

import (
	"container/heap"

	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/maxonlinux/meta-terminal-go/pkg/utils"
)

// CondShard represents a shard for conditional orders within a single symbol shard.
// Each shard maintains separate trigger heaps for BUY and SELL conditional orders,
// allowing concurrent processing of different symbol groups without contention.
//
// BUY triggers use a max-heap: highest trigger price first
// When price drops, the highest trigger (closest from above) fires first.
//
// SELL triggers use a min-heap: lowest trigger price first
// When price rises, the lowest trigger (closest from below) fires first.
type CondShard struct {
	buyTriggers  map[string]*triggerHeap // symbol -> max-heap (highest trigger first)
	sellTriggers map[string]*triggerHeap // symbol -> min-heap (lowest trigger first)
}

// ConditionalIndex manages conditional (trigger) orders with sharding for scalability.
// Supports 1000+ symbols with low contention through 256 shards.
//
// Trigger Logic:
//   - BUY order: Triggers when currentPrice <= triggerPrice (price dropped to/below trigger)
//   - SELL order: Triggers when currentPrice >= triggerPrice (price rose to/above trigger)
//
// Order Cancellation:
//   - Set order.Status = ORDER_STATUS_CANCELED
//   - Cleanup happens lazily during CheckTriggers() via status check
//   - No immediate removal from heap (avoids O(n) heap operations)
type ConditionalIndex struct {
	shards [constants.OMS_SHARD_COUNT]*CondShard
}

// triggerHeap implements heap.Interface for conditional order trigger price comparisons.
// Uses pointer slice since heap.Interface requires []*Item.
//
// BUY triggers: max-heap (highest trigger price first)
// SELL triggers: min-heap (lowest trigger price first)
type triggerHeap struct {
	items []*types.Order
	isBuy bool // Controls heap direction (buy=max, sell=min).
}

// Len returns the number of items in the heap.
// Required by heap.Interface.
func (h triggerHeap) Len() int { return len(h.items) }

// Less compares trigger prices for heap ordering.
// BUY triggers: max-heap (>) - highest trigger price has highest priority
// SELL triggers: min-heap (<) - lowest trigger price has highest priority
func (h triggerHeap) Less(i, j int) bool {
	if h.isBuy {
		return math.Cmp(h.items[i].TriggerPrice, h.items[j].TriggerPrice) > 0
	}
	return math.Cmp(h.items[i].TriggerPrice, h.items[j].TriggerPrice) < 0
}

// Swap exchanges two items in the heap.
// Required by heap.Interface.
func (h triggerHeap) Swap(i, j int) {
	h.items[i], h.items[j] = h.items[j], h.items[i]
}

// Push adds an item to the heap.
// Required by heap.Interface.
func (h *triggerHeap) Push(x any) {
	h.items = append(h.items, x.(*types.Order))
}

// Pop removes and returns the minimum/maximum item from the heap.
// Required by heap.Interface.
func (h *triggerHeap) Pop() any {
	n := len(h.items)
	x := h.items[n-1]
	h.items = h.items[:n-1]
	return x
}

// Peek returns the top item without removing it.
// Performs lazy cleanup of canceled orders. Conditional orders can only be
// UNTRIGGERED (waiting), TRIGGERED (activated), or CANCELED (cancelled).
// They cannot be FILLED before triggering.
func (h *triggerHeap) Peek() *types.Order {
	for h.Len() > 0 {
		o := h.items[0]
		// Lazy cleanup: skip canceled orders
		// Also skip orders that are no longer UNTRIGGERED (shouldn't happen normally)
		if o.Status != constants.ORDER_STATUS_UNTRIGGERED {
			heap.Pop(h)
			continue
		}
		return o
	}
	return nil
}

// IsEmpty returns true if the heap contains no active orders.
func (h *triggerHeap) IsEmpty() bool {
	return h.Len() == 0
}

// NewConditionalIndex creates a new ConditionalIndex with pre-allocated shards.
// Each shard has empty trigger maps that are created on-demand.
func NewConditionalIndex() *ConditionalIndex {
	c := &ConditionalIndex{}
	for i := range c.shards {
		c.shards[i] = &CondShard{
			buyTriggers:  make(map[string]*triggerHeap),
			sellTriggers: make(map[string]*triggerHeap),
		}
	}
	return c
}

// getHeap retrieves an existing trigger heap without creating a new one.
func (c *ConditionalIndex) getHeap(shardIdx uint8, symbol string, isBuy bool) *triggerHeap {
	shard := c.shards[shardIdx]

	if isBuy {
		return shard.buyTriggers[symbol]
	}
	return shard.sellTriggers[symbol]
}

// Add inserts a conditional order into the appropriate trigger heap.
// Only processes orders with a non-zero TriggerPrice.
//
// BUY orders: stored in max-heap order (highest trigger first)
// SELL orders: stored in min-heap order (lowest trigger first)
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
	shard := c.shards[shardIdx]

	var triggerMap map[string]*triggerHeap
	if isBuy {
		triggerMap = shard.buyTriggers
	} else {
		triggerMap = shard.sellTriggers
	}

	if h, ok := triggerMap[symbol]; ok {
		return h
	}

	h := &triggerHeap{isBuy: isBuy}
	triggerMap[symbol] = h
	return h
}

// CheckTriggers checks all triggers for a symbol and invokes callback for each triggered order.
// This uses a single loop pattern: callback is called immediately when order triggers,
// eliminating the need for a second iteration over a returned slice.
//
// BUY triggers fire when currentPrice <= triggerPrice (price dropped to/below trigger).
// BUY uses max-heap: highest trigger fires first (closest from above).
//
// SELL triggers fire when currentPrice >= triggerPrice (price rose to/above trigger).
// SELL uses min-heap: lowest trigger first (closest from below).
//
// The callback is responsible for processing triggered orders (e.g., placing them in the orderbook).
// Status is set to TRIGGERED and UpdatedAt is set to current timestamp.
func (c *ConditionalIndex) CheckTriggers(symbol string, currentPrice types.Price, callback func(*types.Order)) {
	shardIdx := ShardIndex(symbol)

	// Helper to trigger an order: set status, timestamp, invoke callback, pop from heap
	trigger := func(o *types.Order, h *triggerHeap) {
		o.Status = constants.ORDER_STATUS_TRIGGERED
		o.UpdatedAt = utils.NowNano()
		callback(o)
		heap.Pop(h)
	}

	// BUY triggers: fire when currentPrice <= triggerPrice
	// BUY uses max-heap: highest trigger first (front of heap)
	if h := c.getHeap(shardIdx, symbol, true); h != nil {
		for h.Len() > 0 {
			o := h.Peek()
			if o == nil {
				break
			}
			// Early exit: if top trigger doesn't fire, none will
			if math.Cmp(currentPrice, o.TriggerPrice) > 0 {
				break
			}
			trigger(o, h)
		}
	}

	// SELL triggers: fire when currentPrice >= triggerPrice
	// SELL uses min-heap: lowest trigger first (front of heap)
	if h := c.getHeap(shardIdx, symbol, false); h != nil {
		for h.Len() > 0 {
			o := h.Peek()
			if o == nil {
				break
			}
			// Early exit: if top trigger doesn't fire, none will
			if math.Cmp(currentPrice, o.TriggerPrice) < 0 {
				break
			}
			trigger(o, h)
		}
	}
}
