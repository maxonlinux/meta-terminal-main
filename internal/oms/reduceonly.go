package oms

import (
	"container/heap"

	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

type roShard struct {
	exposure map[types.UserID]types.Quantity
}

type ReduceOnlyIndex struct {
	shards    [constants.OMS_SHARD_COUNT]*roShard
	buyHeaps  map[uint8]map[string]*orderHeap
	sellHeaps map[uint8]map[string]*orderHeap
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

func NewReduceOnlyIndex() *ReduceOnlyIndex {
	r := &ReduceOnlyIndex{
		buyHeaps:  make(map[uint8]map[string]*orderHeap),
		sellHeaps: make(map[uint8]map[string]*orderHeap),
	}
	for i := range r.shards {
		r.shards[i] = &roShard{
			exposure: make(map[types.UserID]types.Quantity),
		}
	}
	return r
}

func (r *ReduceOnlyIndex) getOrCreateHeap(shardIdx uint8, symbol string, isBuy bool) *orderHeap {
	symbolMap := r.getHeapMap(shardIdx, isBuy)
	if h, ok := symbolMap[symbol]; ok {
		return h
	}

	h := &orderHeap{}
	symbolMap[symbol] = h
	return h
}

func (r *ReduceOnlyIndex) getHeapMap(shardIdx uint8, isBuy bool) map[string]*orderHeap {
	if isBuy {
		symbolMap := r.buyHeaps[shardIdx]
		if symbolMap == nil {
			symbolMap = make(map[string]*orderHeap)
			r.buyHeaps[shardIdx] = symbolMap
		}
		return symbolMap
	}
	symbolMap := r.sellHeaps[shardIdx]
	if symbolMap == nil {
		symbolMap = make(map[string]*orderHeap)
		r.sellHeaps[shardIdx] = symbolMap
	}
	return symbolMap
}

func (r *ReduceOnlyIndex) Add(o *types.Order) {
	if !o.ReduceOnly {
		return
	}

	shardIdx := ShardIndex(o.Symbol)

	remaining := math.Sub(o.Quantity, o.Filled)
	r.AdjustExposure(o, remaining)

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
	r.AdjustExposure(o, math.Sub(o.Filled, o.Quantity))
}

func (r *ReduceOnlyIndex) AdjustExposure(o *types.Order, delta types.Quantity) {
	shardIdx := ShardIndex(o.Symbol)
	shard := r.shards[shardIdx]
	shard.exposure[o.UserID] = math.Add(shard.exposure[o.UserID], delta)
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
		h = r.sellHeaps[shardIdx][symbol]
	} else {
		h = r.buyHeaps[shardIdx][symbol]
	}

	if h == nil || h.Len() == 0 {
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
			r.AdjustExposure(o, math.Sub(math.Zero, remaining))
			excess = math.Sub(excess, remaining)
			heap.Pop(h)
			continue
		}

		// Partial cancel
		o.Quantity = math.Sub(o.Quantity, excess)
		r.AdjustExposure(o, math.Sub(math.Zero, excess))
		excess = math.Zero
	}
}
