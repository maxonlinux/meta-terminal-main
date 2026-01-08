package orderbook

import (
	"errors"
	"sync"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/domain"
)

var ErrPostOnlyWouldCross = errors.New("post-only would cross")

type level struct {
	price domain.Price
	total domain.Quantity

	head *node
	tail *node

	next *level
	prev *level
}

type node struct {
	order *domain.Order

	next *node
	prev *node

	level *level
}

type OrderBook struct {
	mu sync.RWMutex

	bestBid *level
	bestAsk *level

	bids map[domain.Price]*level
	asks map[domain.Price]*level

	orders map[domain.OrderID]*node
}

func New() *OrderBook {
	return &OrderBook{
		bids:   make(map[domain.Price]*level),
		asks:   make(map[domain.Price]*level),
		orders: make(map[domain.OrderID]*node),
	}
}

func (ob *OrderBook) WouldCross(side int8, price domain.Price) bool {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	if side == constants.ORDER_SIDE_BUY {
		return ob.bestAsk != nil && price >= ob.bestAsk.price
	}
	return ob.bestBid != nil && price <= ob.bestBid.price
}

func (ob *OrderBook) BestBid() (price domain.Price, qty domain.Quantity, ok bool) {
	ob.mu.RLock()
	defer ob.mu.RUnlock()
	if ob.bestBid == nil {
		return 0, 0, false
	}
	return ob.bestBid.price, ob.bestBid.total, true
}

func (ob *OrderBook) BestAsk() (price domain.Price, qty domain.Quantity, ok bool) {
	ob.mu.RLock()
	defer ob.mu.RUnlock()
	if ob.bestAsk == nil {
		return 0, 0, false
	}
	return ob.bestAsk.price, ob.bestAsk.total, true
}

func (ob *OrderBook) Depth(side int8, limit int) []int64 {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	if limit <= 0 {
		return nil
	}

	out := make([]int64, 0, limit*2)
	if side == constants.ORDER_SIDE_BUY {
		for lvl := ob.bestBid; lvl != nil && len(out) < limit*2; lvl = lvl.next {
			out = append(out, int64(lvl.price), int64(lvl.total))
		}
		return out
	}

	for lvl := ob.bestAsk; lvl != nil && len(out) < limit*2; lvl = lvl.next {
		out = append(out, int64(lvl.price), int64(lvl.total))
	}
	return out
}

func (ob *OrderBook) AvailableQuantity(takerSide int8, limitPrice domain.Price, needed domain.Quantity) domain.Quantity {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	var total domain.Quantity
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

func (ob *OrderBook) Match(taker *domain.Order, limitPrice domain.Price) ([]domain.Match, error) {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	if taker.Remaining() <= 0 {
		return nil, nil
	}

	var matches []domain.Match

	if taker.Side == constants.ORDER_SIDE_BUY {
		for taker.Remaining() > 0 && ob.bestAsk != nil {
			lvl := ob.bestAsk
			if limitPrice > 0 && lvl.price > limitPrice {
				break
			}
			matches = ob.matchLevel(taker, lvl, matches)
		}
		return matches, nil
	}

	for taker.Remaining() > 0 && ob.bestBid != nil {
		lvl := ob.bestBid
		if limitPrice > 0 && lvl.price < limitPrice {
			break
		}
		matches = ob.matchLevel(taker, lvl, matches)
	}
	return matches, nil
}

func (ob *OrderBook) matchLevel(taker *domain.Order, lvl *level, matches []domain.Match) []domain.Match {
	for taker.Remaining() > 0 && lvl.head != nil {
		makerNode := lvl.head
		maker := makerNode.order
		makerRemaining := maker.Remaining()
		if makerRemaining <= 0 {
			ob.removeNode(makerNode)
			continue
		}

		exec := makerRemaining
		if exec > taker.Remaining() {
			exec = taker.Remaining()
		}

		maker.Filled += exec
		taker.Filled += exec
		lvl.total -= exec

		trade := &domain.Trade{
			ID:           domain.TradeID(domain.NowNano()),
			Symbol:       taker.Symbol,
			TakerID:      taker.UserID,
			MakerID:      maker.UserID,
			TakerOrderID: taker.ID,
			MakerOrderID: maker.ID,
			Price:        lvl.price,
			Quantity:     exec,
			ExecutedAt:   domain.NowNano(),
		}
		matches = append(matches, domain.Match{Trade: trade, Maker: maker})

		if maker.Remaining() == 0 {
			ob.removeNode(makerNode)
		}
	}
	return matches
}

func (ob *OrderBook) AddResting(order *domain.Order) {
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

func (ob *OrderBook) RemoveResting(orderID domain.OrderID) bool {
	ob.mu.Lock()
	defer ob.mu.Unlock()
	n := ob.orders[orderID]
	if n == nil {
		return false
	}
	ob.removeNode(n)
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
