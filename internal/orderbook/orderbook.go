package orderbook

import (
	"sync/atomic"

	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
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

type bookState struct {
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
	state *bookState // Single-symbol orderbook: state holds all price levels
}

type LevelSnapshot struct {
	Price types.Price
	Total types.Quantity
}

type Snapshot struct {
	Bids []LevelSnapshot
	Asks []LevelSnapshot
}

func New() *OrderBook {
	return &OrderBook{state: newBookState()}
}

func newBookState() *bookState {
	return &bookState{
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

func (sh *bookState) lock() {
	for !atomic.CompareAndSwapInt32(&sh.mu, 0, 1) {
	}
}

func (sh *bookState) unlock() {
	atomic.StoreInt32(&sh.mu, 0)
}

func (sh *bookState) allocLevel() int32 {
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

func (sh *bookState) freeLevel(idx int32) {
	sh.levels[idx].next = sh.levelFree
	sh.levelFree = idx
}

func (sh *bookState) allocNode() int32 {
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

func (sh *bookState) freeNode(idx int32) {
	sh.nodes[idx].next = sh.nodeFree
	sh.nodeFree = idx
}

func remaining(order *types.Order) types.Quantity {
	return order.Quantity.Sub(order.Filled)
}

func (ob *OrderBook) WouldCross(side int8, price types.Price) bool {
	sh := ob.state
	sh.lock()
	defer sh.unlock()

	// Post-only checks need a fast price-cross signal on the best level only.
	if side == constants.ORDER_SIDE_BUY {
		return sh.bestAsk != -1 && price.Cmp(sh.levels[sh.bestAsk].price) >= 0
	}
	return sh.bestBid != -1 && price.Cmp(sh.levels[sh.bestBid].price) <= 0
}

// AvailableQuantity reports how much size can be filled at the taker limit price.
func (ob *OrderBook) AvailableQuantity(takerSide int8, limitPrice types.Price, needed types.Quantity) types.Quantity {
	// Zero or negative requests cannot consume liquidity.
	if needed.Sign() <= 0 {
		return types.Quantity{}
	}

	sh := ob.state
	sh.lock()
	defer sh.unlock()

	var total types.Quantity
	start := sh.bestBid
	limitCmp := -1
	if takerSide == constants.ORDER_SIDE_BUY {
		start = sh.bestAsk
		limitCmp = 1
	}

	for lvlIdx := start; lvlIdx != -1 && total.Cmp(needed) < 0; lvlIdx = sh.levels[lvlIdx].next {
		price := sh.levels[lvlIdx].price
		if limitPrice.Sign() > 0 && price.Cmp(limitPrice) == limitCmp {
			break
		}
		total = total.Add(sh.levels[lvlIdx].total)
	}
	return total
}

// linkLevel inserts a new price level into the side list.
func (sh *bookState) linkLevel(side int8, lvlIdx int32) {
	if side == constants.ORDER_SIDE_BUY {
		sh.insertLevel(lvlIdx, sh.bestBid, true)
		return
	}
	sh.insertLevel(lvlIdx, sh.bestAsk, false)
}

// insertLevel links a level into the sorted list for a side.
func (sh *bookState) insertLevel(lvlIdx int32, head int32, desc bool) {
	cmp := sh.levels[lvlIdx].price.Cmp
	if head == -1 || (desc && cmp(sh.levels[head].price) > 0) || (!desc && cmp(sh.levels[head].price) < 0) {
		sh.levels[lvlIdx].next = head
		if head != -1 {
			sh.levels[head].prev = lvlIdx
		}
		if desc {
			sh.bestBid = lvlIdx
		} else {
			sh.bestAsk = lvlIdx
		}
		return
	}
	cur := head
	next := sh.levels[cur].next
	for next != -1 {
		cmpNext := cmp(sh.levels[next].price)
		if (desc && cmpNext <= 0) || (!desc && cmpNext >= 0) {
			break
		}
		cur = next
		next = sh.levels[cur].next
	}
	sh.levels[lvlIdx].prev = cur
	sh.levels[lvlIdx].next = sh.levels[cur].next
	sh.levels[cur].next = lvlIdx
	if sh.levels[lvlIdx].next != -1 {
		sh.levels[sh.levels[lvlIdx].next].prev = lvlIdx
	}
}

// unlinkLevel removes a price level from the side list.
func (sh *bookState) unlinkLevel(side int8, lvlIdx int32) {
	prev := sh.levels[lvlIdx].prev
	next := sh.levels[lvlIdx].next
	if side == constants.ORDER_SIDE_BUY {
		if prev != -1 {
			sh.levels[prev].next = next
		} else {
			sh.bestBid = next
		}
		if next != -1 {
			sh.levels[next].prev = prev
		}
	} else {
		if prev != -1 {
			sh.levels[prev].next = next
		} else {
			sh.bestAsk = next
		}
		if next != -1 {
			sh.levels[next].prev = prev
		}
	}
	sh.levels[lvlIdx].prev = -1
	sh.levels[lvlIdx].next = -1
}

func (sh *bookState) Add(order *types.Order) {
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
func (sh *bookState) ensureLevel(order *types.Order) (int32, *level) {
	if order.Side == constants.ORDER_SIDE_BUY {
		if existing, ok := sh.bids[order.Price]; ok {
			return existing, &sh.levels[existing]
		}
		lvlIdx := sh.allocLevel()
		sh.levels[lvlIdx].price = order.Price
		sh.bids[order.Price] = lvlIdx
		sh.linkLevel(constants.ORDER_SIDE_BUY, lvlIdx)
		return lvlIdx, &sh.levels[lvlIdx]
	}

	if existing, ok := sh.asks[order.Price]; ok {
		return existing, &sh.levels[existing]
	}
	lvlIdx := sh.allocLevel()
	sh.levels[lvlIdx].price = order.Price
	sh.asks[order.Price] = lvlIdx
	sh.linkLevel(constants.ORDER_SIDE_SELL, lvlIdx)
	return lvlIdx, &sh.levels[lvlIdx]
}

func (ob *OrderBook) Add(order *types.Order) {
	sh := ob.state
	sh.lock()
	defer sh.unlock()
	sh.Add(order)
}

func (sh *bookState) removeNode(nodeIdx int32) {
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
			sh.unlinkLevel(constants.ORDER_SIDE_BUY, lvlIdx)
			sh.freeLevel(lvlIdx)
		} else if existing, ok := sh.asks[price]; ok && existing == lvlIdx {
			delete(sh.asks, price)
			sh.unlinkLevel(constants.ORDER_SIDE_SELL, lvlIdx)
			sh.freeLevel(lvlIdx)
		}
	}

	sh.freeNode(nodeIdx)
}

func (sh *bookState) Remove(orderID types.OrderID) bool {
	nodeIdx, ok := sh.orders[orderID]
	if !ok {
		return false
	}
	sh.removeNode(nodeIdx)
	return true
}

func (sh *bookState) applyFill(orderID types.OrderID, qty types.Quantity) {
	nodeIdx, ok := sh.orders[orderID]
	if !ok {
		return
	}
	n := sh.nodes[nodeIdx]
	lvl := &sh.levels[n.level]
	if qty.Sign() > 0 {
		lvl.total = lvl.total.Sub(qty)
	}
	if remaining(n.order).Sign() <= 0 {
		sh.removeNode(nodeIdx)
	}
}

func (ob *OrderBook) Remove(orderID types.OrderID) bool {
	sh := ob.state
	sh.lock()
	defer sh.unlock()
	return sh.Remove(orderID)
}

func (ob *OrderBook) ApplyFill(orderID types.OrderID, qty types.Quantity) {
	sh := ob.state
	sh.lock()
	defer sh.unlock()
	sh.applyFill(orderID, qty)
}

func (sh *bookState) getMatches(taker *types.Order, limitPrice types.Price, matches []types.Match) []types.Match {
	if remaining(taker).Sign() <= 0 {
		return nil
	}

	rem := remaining(taker)
	limitCmp := -1
	levelIdx := sh.bestBid
	if taker.Side == constants.ORDER_SIDE_BUY {
		limitCmp = 1
		levelIdx = sh.bestAsk
	}

	if matches == nil {
		matches = make([]types.Match, 0, 8)
	} else {
		matches = matches[:0]
	}
	for rem.Sign() > 0 && levelIdx != -1 {
		level := &sh.levels[levelIdx]
		if limitPrice.Sign() > 0 && level.price.Cmp(limitPrice) == limitCmp {
			break
		}
		nodeIdx := level.head
		for nodeIdx != -1 && rem.Sign() > 0 {
			makerNode := sh.nodes[nodeIdx]
			maker := makerNode.order
			makerRemaining := remaining(maker)
			if makerRemaining.Sign() <= 0 {
				nodeIdx = makerNode.next
				continue
			}
			exec := makerRemaining
			if rem.Cmp(makerRemaining) < 0 {
				exec = rem
			}
			matches = append(matches, types.Match{
				Symbol:     taker.Symbol,
				Category:   taker.Category,
				Price:      level.price,
				Quantity:   exec,
				TakerOrder: taker,
				MakerOrder: maker,
			})
			rem = rem.Sub(exec)
			nodeIdx = makerNode.next
		}
		levelIdx = level.next
	}
	return matches
}

func (ob *OrderBook) GetMatches(taker *types.Order, limitPrice types.Price, matches []types.Match) []types.Match {
	sh := ob.state
	sh.lock()
	defer sh.unlock()
	return sh.getMatches(taker, limitPrice, matches)
}

func (ob *OrderBook) Snapshot(limit int) Snapshot {
	sh := ob.state
	if limit <= 0 {
		limit = 50
	}

	bids := make([]LevelSnapshot, 0, limit)
	asks := make([]LevelSnapshot, 0, limit)

	idx := sh.bestBid
	for idx != -1 && len(bids) < limit {
		lvl := sh.levels[idx]
		bids = append(bids, LevelSnapshot{Price: lvl.price, Total: lvl.total})
		idx = lvl.next
	}

	idx = sh.bestAsk
	for idx != -1 && len(asks) < limit {
		lvl := sh.levels[idx]
		asks = append(asks, LevelSnapshot{Price: lvl.price, Total: lvl.total})
		idx = lvl.next
	}

	return Snapshot{Bids: bids, Asks: asks}
}
