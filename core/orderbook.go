package core

import "container/heap"

// OrderBook - heap-based order book, O(1) best bid/ask, O(log n) add/match.
type OrderBook struct {
	lastPrice int64
	bids      *bidHeap
	asks      *askHeap
	orders    map[OrderID]*Order
}

func NewOrderBook() *OrderBook {
	return &OrderBook{
		bids:   &bidHeap{},
		asks:   &askHeap{},
		orders: make(map[OrderID]*Order),
	}
}

func (ob *OrderBook) AddOrder(order *Order) []*Trade {
	ob.orders[order.ID] = order
	if order.Side == ORDER_SIDE_BUY {
		heap.Push(ob.bids, &plEntry{Price: order.Price, Order: order})
	} else {
		heap.Push(ob.asks, &plEntry{Price: order.Price, Order: order})
	}
	return ob.match(order)
}

func (ob *OrderBook) Match(order *Order) []*Trade {
	if order.Side == ORDER_SIDE_BUY {
		return ob.matchAgainstAsks(order)
	}
	return ob.matchAgainstBids(order)
}

func (ob *OrderBook) matchAgainstAsks(order *Order) []*Trade {
	var trades []*Trade
	for ob.asks.Len() > 0 && order.Quantity > 0 {
		ask := (*ob.asks)[0]
		if ask.Price > order.Price {
			break
		}
		if ask.Order.UserID == order.UserID {
			heap.Pop(ob.asks)
			delete(ob.orders, ask.Order.ID)
			continue
		}
		trade := ob.executeTrade(order, ask.Order)
		trades = append(trades, trade)
		if ask.Order.Quantity == 0 {
			heap.Pop(ob.asks)
			delete(ob.orders, ask.Order.ID)
		}
		if order.Quantity == 0 {
			order.Status = ORDER_STATUS_FILLED
		}
	}
	return trades
}

func (ob *OrderBook) matchAgainstBids(order *Order) []*Trade {
	var trades []*Trade
	for ob.bids.Len() > 0 && order.Quantity > 0 {
		bid := (*ob.bids)[0]
		if bid.Price < order.Price {
			break
		}
		if bid.Order.UserID == order.UserID {
			heap.Pop(ob.bids)
			delete(ob.orders, bid.Order.ID)
			continue
		}
		trade := ob.executeTrade(order, bid.Order)
		trades = append(trades, trade)
		if bid.Order.Quantity == 0 {
			heap.Pop(ob.bids)
			delete(ob.orders, bid.Order.ID)
		}
		if order.Quantity == 0 {
			order.Status = ORDER_STATUS_FILLED
		}
	}
	return trades
}

func (ob *OrderBook) executeTrade(taker *Order, maker *Order) *Trade {
	qty := min(taker.Quantity, maker.Quantity)
	trade := &Trade{
		Symbol:     taker.Symbol,
		Price:      maker.Price,
		Quantity:   qty,
		MakerOrder: maker,
		TakerOrder: taker,
		Timestamp:  NowNano(),
	}
	taker.Quantity -= qty
	maker.Quantity -= qty
	if maker.Quantity == 0 {
		maker.Status = ORDER_STATUS_FILLED
	}
	if taker.Quantity == 0 {
		taker.Status = ORDER_STATUS_FILLED
	} else if taker.Filled > 0 {
		taker.Status = ORDER_STATUS_PARTIALLY_FILLED
	}
	return trade
}

func (ob *OrderBook) match(order *Order) []*Trade {
	if order.Side == ORDER_SIDE_BUY {
		return ob.matchAgainstAsks(order)
	}
	return ob.matchAgainstBids(order)
}

func (ob *OrderBook) BestBid() *plEntry {
	if ob.bids.Len() == 0 {
		return nil
	}
	return (*ob.bids)[0]
}

func (ob *OrderBook) BestAsk() *plEntry {
	if ob.asks.Len() == 0 {
		return nil
	}
	return (*ob.asks)[0]
}

func (ob *OrderBook) RemoveOrder(id OrderID) {
	delete(ob.orders, id)
}

type plEntry struct {
	Price Price
	Order *Order
}

type bidHeap []*plEntry

func (h bidHeap) Len() int            { return len(h) }
func (h bidHeap) Less(i, j int) bool  { return h[i].Price > h[j].Price }
func (h bidHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *bidHeap) Push(x interface{}) { *h = append(*h, x.(*plEntry)) }
func (h *bidHeap) Pop() interface{} {
	old := *h
	n := len(old)
	*h = old[0 : n-1]
	return old[n-1]
}

type askHeap []*plEntry

func (h askHeap) Len() int            { return len(h) }
func (h askHeap) Less(i, j int) bool  { return h[i].Price < h[j].Price }
func (h askHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *askHeap) Push(x interface{}) { *h = append(*h, x.(*plEntry)) }
func (h *askHeap) Pop() interface{} {
	old := *h
	n := len(old)
	*h = old[0 : n-1]
	return old[n-1]
}
