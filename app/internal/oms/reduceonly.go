package oms

import (
	"container/heap"

	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

type roShard struct {
	exposure map[roKey]types.Quantity
}

type ReduceOnlyIndex struct {
	shards [constants.OMS_SHARD_COUNT]*roShard
	books  map[uint8]map[string]*reduceOnlyBook
}

type roKey struct {
	userID types.UserID
	symbol string
	side   int8
}

type orderItem struct {
	order    *types.Order
	minIndex int
	maxIndex int
}

type minHeap struct{ items []*orderItem }
type maxHeap struct{ items []*orderItem }

type reduceOnlySide struct {
	items map[types.OrderID]*orderItem
	min   minHeap
	max   maxHeap
}

type reduceOnlyBook struct {
	buy  reduceOnlySide
	sell reduceOnlySide
}

func (h minHeap) Len() int { return len(h.items) }

func (h minHeap) Less(i, j int) bool {
	return math.Cmp(h.items[i].order.Price, h.items[j].order.Price) < 0
}

func (h minHeap) Swap(i, j int) {
	h.items[i], h.items[j] = h.items[j], h.items[i]
	h.items[i].minIndex = i
	h.items[j].minIndex = j
}

func (h *minHeap) Push(x any) {
	item := x.(*orderItem)
	item.minIndex = len(h.items)
	h.items = append(h.items, item)
}

func (h *minHeap) Pop() any {
	n := len(h.items)
	x := h.items[n-1]
	x.minIndex = -1
	h.items = h.items[:n-1]
	return x
}

func (h *minHeap) Peek() *orderItem {
	if h.Len() == 0 {
		return nil
	}
	return h.items[0]
}

func (h maxHeap) Len() int { return len(h.items) }

func (h maxHeap) Less(i, j int) bool {
	return math.Cmp(h.items[i].order.Price, h.items[j].order.Price) > 0
}

func (h maxHeap) Swap(i, j int) {
	h.items[i], h.items[j] = h.items[j], h.items[i]
	h.items[i].maxIndex = i
	h.items[j].maxIndex = j
}

func (h *maxHeap) Push(x any) {
	item := x.(*orderItem)
	item.maxIndex = len(h.items)
	h.items = append(h.items, item)
}

func (h *maxHeap) Pop() any {
	n := len(h.items)
	x := h.items[n-1]
	x.maxIndex = -1
	h.items = h.items[:n-1]
	return x
}

func (h *maxHeap) Peek() *orderItem {
	if h.Len() == 0 {
		return nil
	}
	return h.items[0]
}

func NewReduceOnlyIndex() *ReduceOnlyIndex {
	r := &ReduceOnlyIndex{
		books: make(map[uint8]map[string]*reduceOnlyBook),
	}
	for i := range r.shards {
		r.shards[i] = &roShard{
			exposure: make(map[roKey]types.Quantity),
		}
	}
	return r
}

func (r *ReduceOnlyIndex) getBook(symbol string) *reduceOnlyBook {
	shardIdx := ShardIndex(symbol)
	symbolMap := r.books[shardIdx]
	if symbolMap == nil {
		symbolMap = make(map[string]*reduceOnlyBook)
		r.books[shardIdx] = symbolMap
	}

	book, ok := symbolMap[symbol]
	if !ok {
		book = &reduceOnlyBook{
			buy:  reduceOnlySide{items: make(map[types.OrderID]*orderItem)},
			sell: reduceOnlySide{items: make(map[types.OrderID]*orderItem)},
		}
		symbolMap[symbol] = book
	}
	return book
}

func (b *reduceOnlyBook) side(side int8) *reduceOnlySide {
	if side == constants.ORDER_SIDE_BUY {
		return &b.buy
	}
	return &b.sell
}

func (s *reduceOnlySide) add(item *orderItem) {
	s.items[item.order.ID] = item
	heap.Push(&s.min, item)
	heap.Push(&s.max, item)
}

func (s *reduceOnlySide) remove(item *orderItem) {
	if item.minIndex >= 0 {
		heap.Remove(&s.min, item.minIndex)
	}
	if item.maxIndex >= 0 {
		heap.Remove(&s.max, item.maxIndex)
	}
	delete(s.items, item.order.ID)
}

func (s *reduceOnlySide) peekFarthest(price types.Price) *orderItem {
	minItem := s.min.Peek()
	maxItem := s.max.Peek()
	if minItem == nil {
		return maxItem
	}
	if maxItem == nil {
		return minItem
	}
	minDist := math.AbsFixed(math.Sub(minItem.order.Price, price))
	maxDist := math.AbsFixed(math.Sub(maxItem.order.Price, price))
	if math.Cmp(maxDist, minDist) > 0 {
		return maxItem
	}
	return minItem
}

func (r *ReduceOnlyIndex) Add(o *types.Order) {
	if !o.ReduceOnly {
		return
	}
	remaining := math.Sub(o.Quantity, o.Filled)
	r.adjustExposure(o.UserID, o.Symbol, o.Side, remaining)

	book := r.getBook(o.Symbol)
	item := &orderItem{order: o, minIndex: -1, maxIndex: -1}
	book.side(o.Side).add(item)
}

func (r *ReduceOnlyIndex) Remove(o *types.Order) {
	if o == nil || !o.ReduceOnly {
		return
	}
	remaining := math.Sub(o.Filled, o.Quantity)
	r.adjustExposure(o.UserID, o.Symbol, o.Side, remaining)
	book := r.getBook(o.Symbol)
	item := book.side(o.Side).items[o.ID]
	if item == nil {
		return
	}
	book.side(o.Side).remove(item)
}

func (r *ReduceOnlyIndex) adjustExposure(userID types.UserID, symbol string, side int8, delta types.Quantity) {
	shardIdx := ShardIndex(symbol)
	shard := r.shards[shardIdx]
	key := roKey{userID: userID, symbol: symbol, side: side}
	shard.exposure[key] = math.Add(shard.exposure[key], delta)
}

func (r *ReduceOnlyIndex) OnPositionReduce(userID types.UserID, symbol string, positionSize types.Quantity, price types.Price) {
	shardIdx := ShardIndex(symbol)
	shard := r.shards[shardIdx]
	book := r.getBook(symbol)

	sides := []int8{constants.ORDER_SIDE_SELL}
	if math.Sign(positionSize) < 0 {
		sides = []int8{constants.ORDER_SIDE_BUY}
	} else if math.Sign(positionSize) == 0 {
		sides = []int8{constants.ORDER_SIDE_BUY, constants.ORDER_SIDE_SELL}
	}

	for _, trimSide := range sides {
		key := roKey{userID: userID, symbol: symbol, side: trimSide}
		total := shard.exposure[key]
		allowed := types.Quantity(math.AbsFixed(positionSize))
		if math.Cmp(total, allowed) <= 0 {
			continue
		}
		excess := math.Sub(total, allowed)
		side := book.side(trimSide)
		if side.min.Len() == 0 && side.max.Len() == 0 {
			continue
		}
		for math.Sign(excess) > 0 {
			item := side.peekFarthest(price)
			if item == nil {
				break
			}
			o := item.order
			if o.Status != constants.ORDER_STATUS_NEW && o.Status != constants.ORDER_STATUS_PARTIALLY_FILLED {
				remaining := math.Sub(o.Quantity, o.Filled)
				r.adjustExposure(o.UserID, o.Symbol, o.Side, math.Sub(math.Zero, remaining))
				side.remove(item)
				continue
			}

			remaining := math.Sub(o.Quantity, o.Filled)
			if math.Cmp(remaining, excess) <= 0 {
				o.Status = constants.ORDER_STATUS_CANCELED
				r.adjustExposure(o.UserID, o.Symbol, o.Side, math.Sub(math.Zero, remaining))
				excess = math.Sub(excess, remaining)
				side.remove(item)
				continue
			}

			o.Quantity = math.Sub(o.Quantity, excess)
			r.adjustExposure(o.UserID, o.Symbol, o.Side, math.Sub(math.Zero, excess))
			excess = math.Zero
		}
	}
}
