package prod

import (
	"sync/atomic"

	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/snowflake"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/maxonlinux/meta-terminal-go/pkg/utils"
)

const CACHE_LINE = 64

type level struct {
	price types.Price
	total types.Quantity
	head  int32
	tail  int32
	next  int32
	prev  int32
	_     [CACHE_LINE - 56]byte
}

type node struct {
	order *types.Order
	next  int32
	prev  int32
	level int32
	_     [CACHE_LINE - 32]byte
}

type Shard struct {
	_  [CACHE_LINE]byte
	mu int32
	_  [CACHE_LINE - 8]byte

	bids   map[types.Price]int32
	asks   map[types.Price]int32
	orders map[types.OrderID]int32

	levels []level
	nodes  []node

	levelFree int32
	nodeFree  int32

	bestBid int32
	bestAsk int32
	_       [CACHE_LINE - 8]byte
}

type OrderBook struct {
	shards [256]*Shard
}

func NewShard() *Shard {
	return &Shard{
		bids:      make(map[types.Price]int32),
		asks:      make(map[types.Price]int32),
		orders:    make(map[types.OrderID]int32),
		levels:    make([]level, 0, 64),
		nodes:     make([]node, 0, 256),
		levelFree: -1,
		nodeFree:  -1,
		bestBid:   -1,
		bestAsk:   -1,
	}
}

func New() *OrderBook {
	ob := &OrderBook{}
	for i := range ob.shards {
		ob.shards[i] = NewShard()
	}
	return ob
}

func (ob *OrderBook) shard(symbol string) *Shard {
	var h uint32
	for _, c := range symbol {
		h = h*31 + uint32(c)
	}
	return ob.shards[h%256]
}

func (sh *Shard) lock() {
	for !atomic.CompareAndSwapInt32(&sh.mu, 0, 1) {
	}
}

func (sh *Shard) unlock() {
	atomic.StoreInt32(&sh.mu, 0)
}

func (sh *Shard) allocLevel() int32 {
	if sh.levelFree != -1 {
		idx := sh.levelFree
		sh.levelFree = sh.levels[idx].next
		sh.levels[idx].prev = -1
		sh.levels[idx].head = -1
		sh.levels[idx].tail = -1
		sh.levels[idx].total = types.Quantity{}
		return idx
	}
	idx := int32(len(sh.levels))
	sh.levels = append(sh.levels, level{next: -1, prev: -1, head: -1, tail: -1})
	return idx
}

func (sh *Shard) freeLevel(idx int32) {
	sh.levels[idx].next = sh.levelFree
	sh.levelFree = idx
}

func (sh *Shard) allocNode() int32 {
	if sh.nodeFree != -1 {
		idx := sh.nodeFree
		sh.nodeFree = sh.nodes[idx].next
		sh.nodes[idx].prev = -1
		return idx
	}
	idx := int32(len(sh.nodes))
	sh.nodes = append(sh.nodes, node{next: -1, prev: -1})
	return idx
}

func (sh *Shard) freeNode(idx int32) {
	sh.nodes[idx].next = sh.nodeFree
	sh.nodeFree = idx
}

func remaining(order *types.Order) types.Quantity {
	return order.Quantity.Sub(order.Filled)
}

func (ob *OrderBook) WouldCross(symbol string, side int8, price types.Price) bool {
	sh := ob.shard(symbol)
	sh.lock()
	defer sh.unlock()

	// Post-only checks need a fast price-cross signal on the best level only.
	if side == constants.ORDER_SIDE_BUY {
		return sh.bestAsk != -1 && price.Cmp(sh.levels[sh.bestAsk].price) >= 0
	}
	return sh.bestBid != -1 && price.Cmp(sh.levels[sh.bestBid].price) <= 0
}

func (ob *OrderBook) BestBid(symbol string) (types.Price, types.Quantity, bool) {
	sh := ob.shard(symbol)
	sh.lock()
	defer sh.unlock()

	// Best bid is the highest priced level on the buy side.
	if sh.bestBid == -1 {
		return types.Price{}, types.Quantity{}, false
	}
	return sh.levels[sh.bestBid].price, sh.levels[sh.bestBid].total, true
}

func (ob *OrderBook) BestAsk(symbol string) (types.Price, types.Quantity, bool) {
	sh := ob.shard(symbol)
	sh.lock()
	defer sh.unlock()

	// Best ask is the lowest priced level on the sell side.
	if sh.bestAsk == -1 {
		return types.Price{}, types.Quantity{}, false
	}
	return sh.levels[sh.bestAsk].price, sh.levels[sh.bestAsk].total, true
}

// AvailableQuantity reports how much size can be filled at the taker limit price.
func (ob *OrderBook) AvailableQuantity(symbol string, takerSide int8, limitPrice types.Price, needed types.Quantity) types.Quantity {
	// Zero or negative requests cannot consume liquidity.
	if needed.Sign() <= 0 {
		return types.Quantity{}
	}

	sh := ob.shard(symbol)
	sh.lock()
	defer sh.unlock()

	var total types.Quantity
	if takerSide == constants.ORDER_SIDE_BUY {
		for lvlIdx := sh.bestAsk; lvlIdx != -1 && total.Cmp(needed) < 0; lvlIdx = sh.levels[lvlIdx].next {
			price := sh.levels[lvlIdx].price
			if limitPrice.Sign() > 0 && price.Cmp(limitPrice) > 0 {
				break
			}
			total = total.Add(sh.levels[lvlIdx].total)
		}
		return total
	}

	for lvlIdx := sh.bestBid; lvlIdx != -1 && total.Cmp(needed) < 0; lvlIdx = sh.levels[lvlIdx].next {
		price := sh.levels[lvlIdx].price
		if limitPrice.Sign() > 0 && price.Cmp(limitPrice) < 0 {
			break
		}
		total = total.Add(sh.levels[lvlIdx].total)
	}
	return total
}

func (sh *Shard) linkBid(lvlIdx int32) {
	if sh.bestBid == -1 || sh.levels[lvlIdx].price.Cmp(sh.levels[sh.bestBid].price) > 0 {
		sh.levels[lvlIdx].next = sh.bestBid
		if sh.bestBid != -1 {
			sh.levels[sh.bestBid].prev = lvlIdx
		}
		sh.bestBid = lvlIdx
		return
	}
	cur := sh.bestBid
	for sh.levels[cur].next != -1 && sh.levels[sh.levels[cur].next].price.Cmp(sh.levels[lvlIdx].price) > 0 {
		cur = sh.levels[cur].next
	}
	sh.levels[lvlIdx].prev = cur
	sh.levels[lvlIdx].next = sh.levels[cur].next
	sh.levels[cur].next = lvlIdx
	if sh.levels[lvlIdx].next != -1 {
		sh.levels[sh.levels[lvlIdx].next].prev = lvlIdx
	}
}

func (sh *Shard) unlinkBid(lvlIdx int32) {
	if sh.levels[lvlIdx].prev != -1 {
		sh.levels[sh.levels[lvlIdx].prev].next = sh.levels[lvlIdx].next
	} else {
		sh.bestBid = sh.levels[lvlIdx].next
	}
	if sh.levels[lvlIdx].next != -1 {
		sh.levels[sh.levels[lvlIdx].next].prev = sh.levels[lvlIdx].prev
	}
	sh.levels[lvlIdx].prev = -1
	sh.levels[lvlIdx].next = -1
}

func (sh *Shard) linkAsk(lvlIdx int32) {
	if sh.bestAsk == -1 || sh.levels[lvlIdx].price.Cmp(sh.levels[sh.bestAsk].price) < 0 {
		sh.levels[lvlIdx].next = sh.bestAsk
		if sh.bestAsk != -1 {
			sh.levels[sh.bestAsk].prev = lvlIdx
		}
		sh.bestAsk = lvlIdx
		return
	}
	cur := sh.bestAsk
	for sh.levels[cur].next != -1 && sh.levels[sh.levels[cur].next].price.Cmp(sh.levels[lvlIdx].price) < 0 {
		cur = sh.levels[cur].next
	}
	sh.levels[lvlIdx].prev = cur
	sh.levels[lvlIdx].next = sh.levels[cur].next
	sh.levels[cur].next = lvlIdx
	if sh.levels[lvlIdx].next != -1 {
		sh.levels[sh.levels[lvlIdx].next].prev = lvlIdx
	}
}

func (sh *Shard) unlinkAsk(lvlIdx int32) {
	if sh.levels[lvlIdx].prev != -1 {
		sh.levels[sh.levels[lvlIdx].prev].next = sh.levels[lvlIdx].next
	} else {
		sh.bestAsk = sh.levels[lvlIdx].next
	}
	if sh.levels[lvlIdx].next != -1 {
		sh.levels[sh.levels[lvlIdx].next].prev = sh.levels[lvlIdx].prev
	}
	sh.levels[lvlIdx].prev = -1
	sh.levels[lvlIdx].next = -1
}

func (sh *Shard) Add(order *types.Order) {
	rem := remaining(order)
	if rem.Sign() <= 0 {
		return
	}

	lvlIdx, lvl := sh.ensureLevel(order)
	lvl.total = lvl.total.Add(rem)

	nodeIdx := sh.allocNode()
	sh.nodes[nodeIdx].order = order
	sh.nodes[nodeIdx].level = lvlIdx
	sh.orders[order.ID] = nodeIdx

	if lvl.tail == -1 {
		lvl.head = nodeIdx
		lvl.tail = nodeIdx
		return
	}
	prevIdx := lvl.tail
	lvl.tail = nodeIdx
	sh.nodes[prevIdx].next = nodeIdx
	sh.nodes[nodeIdx].prev = prevIdx
}

// ensureLevel returns the price level for this order, creating and linking if needed.
func (sh *Shard) ensureLevel(order *types.Order) (int32, *level) {
	if order.Side == constants.ORDER_SIDE_BUY {
		if existing, ok := sh.bids[order.Price]; ok {
			return existing, &sh.levels[existing]
		}
		lvlIdx := sh.allocLevel()
		sh.levels[lvlIdx].price = order.Price
		sh.bids[order.Price] = lvlIdx
		sh.linkBid(lvlIdx)
		return lvlIdx, &sh.levels[lvlIdx]
	}

	if existing, ok := sh.asks[order.Price]; ok {
		return existing, &sh.levels[existing]
	}
	lvlIdx := sh.allocLevel()
	sh.levels[lvlIdx].price = order.Price
	sh.asks[order.Price] = lvlIdx
	sh.linkAsk(lvlIdx)
	return lvlIdx, &sh.levels[lvlIdx]
}

func (ob *OrderBook) Add(order *types.Order) {
	sh := ob.shard(order.Symbol)
	sh.lock()
	defer sh.unlock()
	sh.Add(order)
}

func (sh *Shard) removeNode(nodeIdx int32) {
	n := sh.nodes[nodeIdx]
	delete(sh.orders, n.order.ID)
	lvlIdx := n.level
	lvl := &sh.levels[lvlIdx]

	rem := remaining(n.order)
	if rem.Sign() > 0 {
		lvl.total = lvl.total.Sub(rem)
	}

	prevIdx := n.prev
	nextIdx := n.next

	if prevIdx != -1 {
		sh.nodes[prevIdx].next = nextIdx
	} else {
		lvl.head = nextIdx
	}
	if nextIdx != -1 {
		sh.nodes[nextIdx].prev = prevIdx
	} else {
		lvl.tail = prevIdx
	}

	if lvl.head == -1 && lvl.total.Sign() == 0 {
		// Empty levels must be removed from the side map and best pointers.
		price := lvl.price
		if existing, ok := sh.bids[price]; ok && existing == lvlIdx {
			delete(sh.bids, price)
			sh.unlinkBid(lvlIdx)
			sh.freeLevel(lvlIdx)
		} else if existing, ok := sh.asks[price]; ok && existing == lvlIdx {
			delete(sh.asks, price)
			sh.unlinkAsk(lvlIdx)
			sh.freeLevel(lvlIdx)
		}
	}

	sh.freeNode(nodeIdx)
}

func (sh *Shard) Remove(orderID types.OrderID) bool {
	nodeIdx, ok := sh.orders[orderID]
	if !ok {
		return false
	}
	sh.removeNode(nodeIdx)
	return true
}

func (ob *OrderBook) Remove(symbol string, orderID types.OrderID) bool {
	sh := ob.shard(symbol)
	sh.lock()
	defer sh.unlock()
	return sh.Remove(orderID)
}

func (sh *Shard) matchLevel(taker *types.Order, lvlIdx int32, emit func(types.Trade)) {
	for remaining(taker).Sign() > 0 && sh.levels[lvlIdx].head != -1 {
		makerNodeIdx := sh.levels[lvlIdx].head
		maker := sh.nodes[makerNodeIdx].order
		makerRemaining := remaining(maker)

		if makerRemaining.Sign() <= 0 {
			sh.removeNode(makerNodeIdx)
			continue
		}

		exec := makerRemaining
		if remaining(taker).Cmp(makerRemaining) < 0 {
			exec = remaining(taker)
		}

		maker.Filled = maker.Filled.Add(exec)
		taker.Filled = taker.Filled.Add(exec)
		sh.levels[lvlIdx].total = sh.levels[lvlIdx].total.Sub(exec)

		trade := types.Trade{
			ID:           types.TradeID(snowflake.Next()),
			Symbol:       taker.Symbol,
			Category:     taker.Category,
			TakerOrder:   taker,
			MakerOrder:   maker,
			TakerOrderID: taker.ID,
			MakerOrderID: maker.ID,
			TakerID:      taker.UserID,
			MakerID:      maker.UserID,
			Price:        sh.levels[lvlIdx].price,
			Quantity:     exec,
			Timestamp:    utils.NowNano(),
		}
		emit(trade)

		if remaining(maker).Sign() == 0 {
			sh.removeNode(makerNodeIdx)
		}
	}
}

func (sh *Shard) matchInto(taker *types.Order, limitPrice types.Price, emit func(types.Trade)) {
	if taker.Side == constants.ORDER_SIDE_BUY {
		for remaining(taker).Sign() > 0 && sh.bestAsk != -1 {
			lvlIdx := sh.bestAsk
			if limitPrice.Sign() > 0 && sh.levels[lvlIdx].price.Cmp(limitPrice) > 0 {
				break
			}
			sh.matchLevel(taker, lvlIdx, emit)
		}
		return
	}

	for remaining(taker).Sign() > 0 && sh.bestBid != -1 {
		lvlIdx := sh.bestBid
		if limitPrice.Sign() > 0 && sh.levels[lvlIdx].price.Cmp(limitPrice) < 0 {
			break
		}
		sh.matchLevel(taker, lvlIdx, emit)
	}
}

type TradeHandler func(trade types.Trade)

func (sh *Shard) Match(taker *types.Order, limitPrice types.Price, handler TradeHandler) {
	if remaining(taker).Sign() <= 0 {
		return
	}
	sh.matchInto(taker, limitPrice, handler)
}

func (ob *OrderBook) Match(symbol string, taker *types.Order, limitPrice types.Price, handler TradeHandler) {
	sh := ob.shard(symbol)
	sh.lock()
	defer sh.unlock()
	sh.Match(taker, limitPrice, handler)
}
