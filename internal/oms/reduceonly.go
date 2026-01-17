package oms

import (
	"container/heap"

	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

type ROShard struct {
	buyHeap  *orderHeap
	sellHeap *orderHeap
	exposure map[types.UserID]types.Quantity
}

type ReduceOnlyIndex struct {
	shards      [constants.OMS_SHARD_COUNT]*ROShard
	symbolHeaps map[uint8]map[string]*orderHeap // shard -> symbol -> heap
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

func (h *orderHeap) IsEmpty() bool {
	return h.Len() == 0
}

func NewReduceOnlyManager() *ReduceOnlyIndex {
	r := &ReduceOnlyIndex{
		symbolHeaps: make(map[uint8]map[string]*orderHeap),
	}
	for i := range r.shards {
		r.shards[i] = &ROShard{
			exposure: make(map[types.UserID]types.Quantity),
		}
	}
	return r
}

func (r *ReduceOnlyIndex) getOrCreateHeap(shardIdx uint8, symbol string, isBuy bool) *orderHeap {
	symbolMap := r.symbolHeaps[shardIdx]
	if symbolMap == nil {
		symbolMap = make(map[string]*orderHeap)
		r.symbolHeaps[shardIdx] = symbolMap
	}

	key := symbol
	if !isBuy {
		key = "SELL:" + symbol // Separate key for sell heap
	}

	if h, ok := symbolMap[key]; ok {
		return h
	}

	h := &orderHeap{}
	symbolMap[key] = h
	return h
}

func (r *ReduceOnlyIndex) Add(o *types.Order) {
	if !o.ReduceOnly {
		return
	}

	shardIdx := ShardIndex(o.Symbol)
	shard := r.shards[shardIdx]

	remaining := math.Sub(o.Quantity, o.Filled)
	shard.exposure[o.UserID] = math.Add(shard.exposure[o.UserID], remaining)

	var h *orderHeap
	switch o.Side {
	case constants.ORDER_SIDE_BUY:
		h = r.getOrCreateHeap(shardIdx, o.Symbol, true)
	case constants.ORDER_SIDE_SELL:
		h = r.getOrCreateHeap(shardIdx, o.Symbol, false)
	default:
		return
	}
	heap.Push(h, o)
}

func (r *ReduceOnlyIndex) Remove(o *types.Order) {
	shardIdx := ShardIndex(o.Symbol)
	shard := r.shards[shardIdx]

	remaining := math.Sub(o.Quantity, o.Filled)
	shard.exposure[o.UserID] = math.Sub(shard.exposure[o.UserID], remaining)
}

func (r *ReduceOnlyIndex) OnPositionReduce(symbol string, positionSize types.Quantity, userID types.UserID) {
	shardIdx := ShardIndex(symbol)
	shard := r.shards[shardIdx]

	total := shard.exposure[userID]
	if math.Cmp(total, positionSize) <= 0 {
		return
	}

	excess := math.Sub(total, positionSize)

	// Choose heap based on position direction
	// LONG position (size > 0): cancel SELL orders (farthest down = highest price)
	// SHORT position (size < 0): cancel BUY orders (farthest up = lowest price)
	var h *orderHeap
	if positionSize.Sign() > 0 {
		key := "SELL:" + symbol
		if m, ok := r.symbolHeaps[shardIdx]; ok {
			h = m[key]
		}
	} else {
		// Guard missing shard map to avoid nil panics on cold symbols.
		if m, ok := r.symbolHeaps[shardIdx]; ok {
			h = m[symbol]
		}
	}

	if h == nil || h.IsEmpty() {
		return
	}

	for excess.Sign() > 0 {
		o := h.Peek()
		if o == nil {
			break
		}

		// Cleanup already filled/canceled orders using status
		if o.Status == constants.ORDER_STATUS_FILLED ||
			o.Status == constants.ORDER_STATUS_CANCELED ||
			o.Status == constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED {
			heap.Pop(h)
			continue
		}

		remaining := math.Sub(o.Quantity, o.Filled)
		if math.Cmp(remaining, excess) <= 0 {
			// Cancel entire order
			o.Status = constants.ORDER_STATUS_CANCELED
			shard.exposure[userID] = math.Sub(shard.exposure[userID], remaining)
			excess = math.Sub(excess, remaining)
			heap.Pop(h)
			continue
		}

		// Partial cancel
		o.Quantity = math.Sub(o.Quantity, excess)
		shard.exposure[userID] = math.Sub(shard.exposure[userID], excess)
		excess = math.Zero
	}
}
