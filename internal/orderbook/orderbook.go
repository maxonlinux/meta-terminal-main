package orderbook

import (
	"sync"

	"github.com/maxonlinux/meta-terminal-go/internal/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/snowflake"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

// matchSlicePool provides zero-allocation slice recycling for Match operations.
// Prevents heap allocations when returning []types.Match from Match() function.
// Retrieved slices have capacity 8, which covers most single-order match scenarios.
// Uses pointer to avoid allocations on Put.
var matchSlicePool = sync.Pool{
	New: func() interface{} {
		buf := make([]types.Match, 0, 8)
		return &buf
	},
}

type IDGenerator interface {
	Next() int64
}

type level struct {
	price types.Price
	total types.Quantity

	head *node
	tail *node

	next *level
	prev *level
}

type node struct {
	order *types.Order

	next *node
	prev *node

	level *level
}

type OrderBook struct {
	mu sync.RWMutex

	bestBid *level
	bestAsk *level

	bids map[types.Price]*level
	asks map[types.Price]*level

	orders map[types.OrderID]*node

	idGen IDGenerator
}

func New() *OrderBook {
	return NewWithIDGenerator(nil)
}

func NewWithIDGenerator(idGen IDGenerator) *OrderBook {
	return &OrderBook{
		bids:   make(map[types.Price]*level),
		asks:   make(map[types.Price]*level),
		orders: make(map[types.OrderID]*node),
		idGen:  idGen,
	}
}

func (ob *OrderBook) SetIDGenerator(idGen IDGenerator) {
	ob.mu.Lock()
	defer ob.mu.Unlock()
	ob.idGen = idGen
}

func (ob *OrderBook) WouldCross(side int8, price types.Price) bool {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	if side == constants.ORDER_SIDE_BUY {
		return ob.bestAsk != nil && price >= ob.bestAsk.price
	}
	return ob.bestBid != nil && price <= ob.bestBid.price
}

func (ob *OrderBook) BestBid() (price types.Price, qty types.Quantity, ok bool) {
	ob.mu.RLock()
	defer ob.mu.RUnlock()
	if ob.bestBid == nil {
		return 0, 0, false
	}
	return ob.bestBid.price, ob.bestBid.total, true
}

func (ob *OrderBook) BestAsk() (price types.Price, qty types.Quantity, ok bool) {
	ob.mu.RLock()
	defer ob.mu.RUnlock()
	if ob.bestAsk == nil {
		return 0, 0, false
	}
	return ob.bestAsk.price, ob.bestAsk.total, true
}

func (ob *OrderBook) Depth(limit int) ([]types.Price, []types.Quantity, []types.Price, []types.Quantity) {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	if limit <= 0 {
		return nil, nil, nil, nil
	}

	bidPrices := make([]types.Price, 0, limit)
	bidQtys := make([]types.Quantity, 0, limit)

	for lvl := ob.bestBid; lvl != nil && len(bidPrices) < limit; lvl = lvl.next {
		bidPrices = append(bidPrices, lvl.price)
		bidQtys = append(bidQtys, lvl.total)
	}

	askPrices := make([]types.Price, 0, limit)
	askQtys := make([]types.Quantity, 0, limit)

	for lvl := ob.bestAsk; lvl != nil && len(askPrices) < limit; lvl = lvl.next {
		askPrices = append(askPrices, lvl.price)
		askQtys = append(askQtys, lvl.total)
	}

	return bidPrices, bidQtys, askPrices, askQtys
}

func (ob *OrderBook) AvailableQuantity(takerSide int8, limitPrice types.Price, needed types.Quantity) types.Quantity {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	var total types.Quantity
	if needed <= 0 {
		return 0
	}

	if takerSide == constants.ORDER_SIDE_BUY {
		for lvl := ob.bestAsk; lvl != nil && total < needed; lvl = lvl.next {
			if limitPrice > 0 && lvl.price > limitPrice {
				break
			}
			total += lvl.total
		}
		return total
	}

	for lvl := ob.bestBid; lvl != nil && total < needed; lvl = lvl.next {
		if limitPrice > 0 && lvl.price < limitPrice {
			break
		}
		total += lvl.total
	}
	return total
}

func (ob *OrderBook) Match(taker *types.Order, limitPrice types.Price) ([]types.Match, error) {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	if taker.Remaining() <= 0 {
		return nil, nil
	}

	return ob.matchInto(taker, limitPrice, nil), nil
}

func (ob *OrderBook) MatchInto(taker *types.Order, limitPrice types.Price, out []types.Match) ([]types.Match, error) {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	if taker.Remaining() <= 0 {
		if out != nil {
			return out[:0], nil
		}
		return nil, nil
	}

	return ob.matchInto(taker, limitPrice, out), nil
}

func (ob *OrderBook) matchInto(taker *types.Order, limitPrice types.Price, out []types.Match) []types.Match {
	// matches holds the result slice. We track whether it came from our pool
	// to ensure we return it to the pool after use (unless caller provided their own).
	var matches []types.Match
	didNotOriginateFromPool := false
	if out != nil {
		// Caller provided their own buffer - reuse it without pooling.
		// This is the MatchInto() path where caller manages memory lifecycle.
		matches = out[:0]
		didNotOriginateFromPool = true
	} else {
		// Fast path: get recycled slice from pool for zero-allocation.
		// Pool provides pre-allocated []types.Match with capacity 8.
		matchesBuf := matchSlicePool.Get().(*[]types.Match)
		matches = (*matchesBuf)[:0]
	}

	// Matching loop - same logic for BUY (asks) and SELL (bids) sides.
	if taker.Side == constants.ORDER_SIDE_BUY {
		for taker.Remaining() > 0 && ob.bestAsk != nil {
			lvl := ob.bestAsk
			if limitPrice > 0 && lvl.price > limitPrice {
				break
			}
			matches = ob.matchLevel(taker, lvl, matches)
		}
		// Return slice to pool if it came from pool (caller didn't provide their own).
		if !didNotOriginateFromPool {
			matchSlicePool.Put(&matches)
		}
		return matches
	}

	for taker.Remaining() > 0 && ob.bestBid != nil {
		lvl := ob.bestBid
		if limitPrice > 0 && lvl.price < limitPrice {
			break
		}
		matches = ob.matchLevel(taker, lvl, matches)
	}
	if !didNotOriginateFromPool {
		matchSlicePool.Put(&matches)
	}
	return matches
}

func (ob *OrderBook) matchLevel(taker *types.Order, lvl *level, matches []types.Match) []types.Match {
	for taker.Remaining() > 0 && lvl.head != nil {
		makerNode := lvl.head
		maker := makerNode.order
		makerRemaining := maker.Remaining()
		if makerRemaining <= 0 {
			ob.removeNode(makerNode)
			continue
		}

		exec := makerRemaining
		if taker.Remaining() < makerRemaining {
			exec = taker.Remaining()
		}

		maker.Filled += exec
		taker.Filled += exec
		lvl.total -= exec

		trade := types.Trade{
			ID:           types.TradeID(ob.nextTradeID()),
			Symbol:       taker.Symbol,
			Category:     taker.Category,
			TakerID:      taker.UserID,
			MakerID:      maker.UserID,
			TakerOrderID: taker.ID,
			MakerOrderID: maker.ID,
			Price:        lvl.price,
			Quantity:     exec,
			ExecutedAt:   types.NowNano(),
		}
		matches = append(matches, types.Match{Trade: trade, Maker: maker})

		if maker.Remaining() == 0 {
			ob.removeNode(makerNode)
		}
	}
	return matches
}

func (ob *OrderBook) nextTradeID() int64 {
	if ob.idGen != nil {
		return ob.idGen.Next()
	}
	return snowflake.Next()
}

func (ob *OrderBook) Add(order *types.Order) {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	remaining := order.Remaining()
	if remaining <= 0 {
		return
	}

	var lvl *level
	if order.Side == constants.ORDER_SIDE_BUY {
		lvl = ob.bids[order.Price]
		if lvl == nil {
			lvl = &level{price: order.Price}
			ob.bids[order.Price] = lvl
			ob.linkBid(lvl)
		}
	} else {
		lvl = ob.asks[order.Price]
		if lvl == nil {
			lvl = &level{price: order.Price}
			ob.asks[order.Price] = lvl
			ob.linkAsk(lvl)
		}
	}

	lvl.total += remaining
	n := &node{order: order, level: lvl}
	ob.orders[order.ID] = n
	if lvl.tail == nil {
		lvl.head = n
		lvl.tail = n
		return
	}
	n.prev = lvl.tail
	lvl.tail.next = n
	lvl.tail = n
}

func (ob *OrderBook) Remove(orderID types.OrderID) bool {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	n := ob.orders[orderID]
	if n == nil {
		return false
	}
	ob.removeNode(n)
	return true
}

func (ob *OrderBook) Adjust(orderID types.OrderID, newRemaining types.Quantity) bool {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	n := ob.orders[orderID]
	if n == nil || n.order == nil {
		return false
	}

	curRemaining := n.order.Remaining()
	if newRemaining != curRemaining {
		delta := newRemaining - curRemaining
		n.level.total += delta
	}
	n.order.Quantity = n.order.Filled + newRemaining
	return true
}

func (ob *OrderBook) removeNode(n *node) {
	delete(ob.orders, n.order.ID)
	lvl := n.level

	remaining := n.order.Remaining()
	if remaining > 0 {
		lvl.total -= remaining
	}

	if n.prev != nil {
		n.prev.next = n.next
	} else {
		lvl.head = n.next
	}
	if n.next != nil {
		n.next.prev = n.prev
	} else {
		lvl.tail = n.prev
	}

	n.prev = nil
	n.next = nil
	n.level = nil

	if lvl.head == nil && lvl.total == 0 {
		if lvl == ob.bestBid {
			ob.unlinkBid(lvl)
			delete(ob.bids, lvl.price)
		} else if lvl == ob.bestAsk {
			ob.unlinkAsk(lvl)
			delete(ob.asks, lvl.price)
		} else if _, ok := ob.bids[lvl.price]; ok {
			ob.unlinkBid(lvl)
			delete(ob.bids, lvl.price)
		} else if _, ok := ob.asks[lvl.price]; ok {
			ob.unlinkAsk(lvl)
			delete(ob.asks, lvl.price)
		}
	}
}

func (ob *OrderBook) linkBid(lvl *level) {
	if ob.bestBid == nil || lvl.price > ob.bestBid.price {
		lvl.next = ob.bestBid
		if ob.bestBid != nil {
			ob.bestBid.prev = lvl
		}
		ob.bestBid = lvl
		return
	}
	cur := ob.bestBid
	for cur.next != nil && cur.next.price > lvl.price {
		cur = cur.next
	}
	lvl.next = cur.next
	lvl.prev = cur
	if cur.next != nil {
		cur.next.prev = lvl
	}
	cur.next = lvl
}

func (ob *OrderBook) unlinkBid(lvl *level) {
	if lvl.prev != nil {
		lvl.prev.next = lvl.next
	} else {
		ob.bestBid = lvl.next
	}
	if lvl.next != nil {
		lvl.next.prev = lvl.prev
	}
	lvl.prev = nil
	lvl.next = nil
}

func (ob *OrderBook) linkAsk(lvl *level) {
	if ob.bestAsk == nil || lvl.price < ob.bestAsk.price {
		lvl.next = ob.bestAsk
		if ob.bestAsk != nil {
			ob.bestAsk.prev = lvl
		}
		ob.bestAsk = lvl
		return
	}
	cur := ob.bestAsk
	for cur.next != nil && cur.next.price < lvl.price {
		cur = cur.next
	}
	lvl.next = cur.next
	lvl.prev = cur
	if cur.next != nil {
		cur.next.prev = lvl
	}
	cur.next = lvl
}

func (ob *OrderBook) unlinkAsk(lvl *level) {
	if lvl.prev != nil {
		lvl.prev.next = lvl.next
	} else {
		ob.bestAsk = lvl.next
	}
	if lvl.next != nil {
		lvl.next.prev = lvl.prev
	}
	lvl.prev = nil
	lvl.next = nil
}
