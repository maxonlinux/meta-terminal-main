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
	shards  [constants.OMS_SHARD_COUNT]*roShard
	heaps   map[uint8]map[string]*orderHeap
	deleted map[types.OrderID]bool
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
		heaps:   make(map[uint8]map[string]*orderHeap),
		deleted: make(map[types.OrderID]bool),
	}
	for i := range r.shards {
		r.shards[i] = &roShard{
			exposure: make(map[types.UserID]types.Quantity),
		}
	}
	return r
}

func (r *ReduceOnlyIndex) getHeap(symbol string, isBuy bool) *orderHeap {
	shardIdx := ShardIndex(symbol)
	symbolMap := r.heaps[shardIdx]
	if symbolMap == nil {
		symbolMap = make(map[string]*orderHeap)
		r.heaps[shardIdx] = symbolMap
	}

	h, ok := symbolMap[symbol]
	if !ok {
		h = &orderHeap{}
		symbolMap[symbol] = h
	}
	return h
}

func (r *ReduceOnlyIndex) Add(o *types.Order) {
	if !o.ReduceOnly {
		return
	}

	delete(r.deleted, o.ID)

	remaining := math.Sub(o.Quantity, o.Filled)
	r.adjustExposure(o.UserID, o.Symbol, remaining)

	h := r.getHeap(o.Symbol, o.Side == constants.ORDER_SIDE_BUY)
	heap.Push(h, o)
}

func (r *ReduceOnlyIndex) Remove(o *types.Order) {
	remaining := math.Sub(o.Filled, o.Quantity)
	r.adjustExposure(o.UserID, o.Symbol, remaining)
	r.deleted[o.ID] = true
}

func (r *ReduceOnlyIndex) adjustExposure(userID types.UserID, symbol string, delta types.Quantity) {
	shardIdx := ShardIndex(symbol)
	shard := r.shards[shardIdx]
	shard.exposure[userID] = math.Add(shard.exposure[userID], delta)
}

func (r *ReduceOnlyIndex) OnPositionReduce(userID types.UserID, symbol string, positionSize types.Quantity) {
	shardIdx := ShardIndex(symbol)
	shard := r.shards[shardIdx]

	total := shard.exposure[userID]
	if math.Cmp(total, positionSize) <= 0 {
		return
	}

	excess := math.Sub(total, positionSize)

	isBuy := positionSize.Sign() < 0
	h := r.getHeap(symbol, isBuy)

	if h.Len() == 0 {
		return
	}

	for excess.Sign() > 0 {
		o := h.Peek()
		if o == nil {
			break
		}

		if r.deleted[o.ID] {
			heap.Pop(h)
			delete(r.deleted, o.ID)
			continue
		}

		if o.Status != constants.ORDER_STATUS_NEW && o.Status != constants.ORDER_STATUS_PARTIALLY_FILLED {
			heap.Pop(h)
			continue
		}

		remaining := math.Sub(o.Quantity, o.Filled)
		if math.Cmp(remaining, excess) <= 0 {
			o.Status = constants.ORDER_STATUS_CANCELED
			r.adjustExposure(o.UserID, o.Symbol, math.Sub(math.Zero, remaining))
			excess = math.Sub(excess, remaining)
			heap.Pop(h)
			continue
		}

		o.Quantity = math.Sub(o.Quantity, excess)
		r.adjustExposure(o.UserID, o.Symbol, math.Sub(math.Zero, excess))
		excess = math.Zero
	}
}
