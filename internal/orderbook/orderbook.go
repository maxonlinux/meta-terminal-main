package orderbook

import (
	"container/heap"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/types"
)

type OrderBook struct {
	category   int8
	orderStore *state.OrderStore
	bids       *PriceHeap
	asks       *PriceHeap
}

type PriceHeap struct {
	data  []*PriceLevel
	isBid bool
}

type PriceLevel struct {
	Price          types.Price
	Quantity       types.Quantity
	Orders         *state.OrderHeap
	PrevPriceLevel *PriceLevel
	NextPriceLevel *PriceLevel
}

func New(category int8, orderStore *state.OrderStore) *OrderBook {
	return &OrderBook{
		category:   category,
		orderStore: orderStore,
		bids:       &PriceHeap{data: make([]*PriceLevel, 0), isBid: true},
		asks:       &PriceHeap{data: make([]*PriceLevel, 0), isBid: false},
	}
}

func (h *PriceHeap) Len() int { return len(h.data) }

func (h *PriceHeap) Less(i, j int) bool {
	if h.isBid {
		return h.data[i].Price > h.data[j].Price
	}
	return h.data[i].Price < h.data[j].Price
}

func (h *PriceHeap) Swap(i, j int) {
	h.data[i], h.data[j] = h.data[j], h.data[i]
}

func (h *PriceHeap) Push(x interface{}) {
	level := x.(*PriceLevel)
	h.data = append(h.data, level)
}

func (h *PriceHeap) Pop() interface{} {
	if len(h.data) == 0 {
		return nil
	}
	last := h.data[len(h.data)-1]
	h.data = h.data[:len(h.data)-1]
	return last
}

func (h *PriceHeap) Peek() *PriceLevel {
	if len(h.data) == 0 {
		return nil
	}
	return h.data[0]
}

func (h *PriceHeap) GetOrCreate(price types.Price) *PriceLevel {
	for _, level := range h.data {
		if level.Price == price {
			return level
		}
	}
	level := &PriceLevel{
		Price:  price,
		Orders: state.NewOrderHeap(),
	}
	heap.Push(h, level)
	return level
}

func (h *PriceHeap) RemovePrice(price types.Price) {
	for i, level := range h.data {
		if level.Price == price {
			h.data = append(h.data[:i], h.data[i+1:]...)
			return
		}
	}
}

func (ob *OrderBook) AddOrder(order *types.Order) ([]*types.Trade, error) {
	ob.orderStore.Add(order)

	var trades []*types.Trade
	var remaining types.Quantity

	switch order.TIF {
	case constants.TIF_GTC, constants.TIF_POST_ONLY:
		trades, remaining = ob.match(order)
		order.Filled = order.Quantity - remaining

		if remaining > 0 {
			ob.addToBook(order)
			order.Status = constants.ORDER_STATUS_NEW
		} else if len(trades) > 0 {
			order.Status = constants.ORDER_STATUS_FILLED
		}

	case constants.TIF_IOC:
		trades, remaining = ob.match(order)
		order.Filled = order.Quantity - remaining
		if remaining > 0 {
			order.Status = constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED
		} else if len(trades) > 0 {
			order.Status = constants.ORDER_STATUS_FILLED
		} else {
			order.Status = constants.ORDER_STATUS_CANCELED
		}

	case constants.TIF_FOK:
		trades, remaining = ob.match(order)
		if remaining > 0 {
			for _, t := range trades {
				ob.reverseTrade(t)
			}
			order.Status = constants.ORDER_STATUS_CANCELED
		} else {
			order.Filled = order.Quantity
			order.Status = constants.ORDER_STATUS_FILLED
		}
	}

	return trades, nil
}

func (ob *OrderBook) match(order *types.Order) ([]*types.Trade, types.Quantity) {
	var trades []*types.Trade
	remaining := order.Quantity

	for remaining > 0 {
		var bestLevel *PriceLevel

		if order.Side == constants.ORDER_SIDE_BUY {
			bestLevel = ob.asks.Peek()
			if bestLevel == nil {
				break
			}
			if order.Price > 0 && order.Price < bestLevel.Price {
				break
			}
		} else {
			bestLevel = ob.bids.Peek()
			if bestLevel == nil {
				break
			}
			if order.Price > 0 && order.Price > bestLevel.Price {
				break
			}
		}

		filled := ob.matchAtLevel(order, bestLevel, remaining, &trades)
		remaining -= filled

		if bestLevel.Orders.Len() == 0 || bestLevel.Quantity == 0 {
			if order.Side == constants.ORDER_SIDE_BUY {
				heap.Pop(ob.asks)
			} else {
				heap.Pop(ob.bids)
			}
		}

		if filled == 0 {
			break
		}
	}

	return trades, remaining
}

func (ob *OrderBook) matchAtLevel(taker *types.Order, level *PriceLevel, remaining types.Quantity, trades *[]*types.Trade) types.Quantity {
	var filled types.Quantity
	tradePrice := level.Price

	for remaining > filled {
		if level.Orders.Len() == 0 {
			break
		}

		makerOID := level.Orders.Peek()
		if makerOID == 0 {
			break
		}

		maker := ob.orderStore.Get(makerOID)
		if maker == nil || maker.Status == constants.ORDER_STATUS_FILLED || maker.Status == constants.ORDER_STATUS_CANCELED {
			level.Orders.Pop()
			continue
		}

		available := maker.Quantity - maker.Filled
		if available <= 0 {
			level.Orders.Pop()
			continue
		}

		tradeQty := remaining - filled
		if tradeQty > available {
			tradeQty = available
		}

		trade := &types.Trade{
			TakerOrderID: taker.ID,
			MakerOrderID: makerOID,
			Price:        tradePrice,
			Quantity:     tradeQty,
			ExecutedAt:   types.NanoTime(),
		}
		*trades = append(*trades, trade)

		maker.Filled += tradeQty
		taker.Filled += tradeQty
		level.Quantity -= tradeQty
		filled += tradeQty

		if maker.Filled >= maker.Quantity {
			maker.Status = constants.ORDER_STATUS_FILLED
			level.Orders.Pop()
		}
	}

	return filled
}

func (ob *OrderBook) addToBook(order *types.Order) {
	var heap *PriceHeap
	if order.Side == constants.ORDER_SIDE_BUY {
		heap = ob.bids
	} else {
		heap = ob.asks
	}

	level := heap.GetOrCreate(order.Price)
	level.Quantity += order.Quantity - order.Filled
	level.Orders.Push(order.ID)
}

func (ob *OrderBook) RemoveOrder(order *types.Order) {
	if order.Status != constants.ORDER_STATUS_NEW {
		return
	}

	remaining := order.Quantity - order.Filled

	var level *PriceLevel
	if order.Side == constants.ORDER_SIDE_BUY {
		for _, l := range ob.bids.data {
			if l.Price == order.Price {
				level = l
				break
			}
		}
	} else {
		for _, l := range ob.asks.data {
			if l.Price == order.Price {
				level = l
				break
			}
		}
	}

	if level == nil {
		return
	}

	level.Quantity -= remaining
	level.Orders.Remove(order.ID)

	if level.Orders.Len() == 0 {
		if order.Side == constants.ORDER_SIDE_BUY {
			ob.bids.RemovePrice(order.Price)
		} else {
			ob.asks.RemovePrice(order.Price)
		}
	}
}

func (ob *OrderBook) WouldCross(order *types.Order) bool {
	if order.Side == constants.ORDER_SIDE_BUY {
		bestAsk := ob.asks.Peek()
		return bestAsk != nil && order.Price >= bestAsk.Price
	}
	bestBid := ob.bids.Peek()
	return bestBid != nil && order.Price <= bestBid.Price
}

func (ob *OrderBook) reverseTrade(trade *types.Trade) {
	taker := ob.orderStore.Get(trade.TakerOrderID)
	maker := ob.orderStore.Get(trade.MakerOrderID)

	if taker != nil {
		taker.Filled -= trade.Quantity
	}
	if maker != nil {
		maker.Filled -= trade.Quantity
	}
}

func (ob *OrderBook) GetBids() []*PriceLevel {
	return ob.bids.data
}

func (ob *OrderBook) GetAsks() []*PriceLevel {
	return ob.asks.data
}

func (ob *OrderBook) GetBestBid() *PriceLevel {
	return ob.bids.Peek()
}

func (ob *OrderBook) GetBestAsk() *PriceLevel {
	return ob.asks.Peek()
}

func (ob *OrderBook) GetDepth(side int8, limit int) []int64 {
	if side == constants.ORDER_SIDE_BUY {
		n := len(ob.bids.data)
		if n > limit {
			n = limit
		}
		depth := make([]int64, 0, n*2)
		for i := 0; i < n; i++ {
			level := ob.bids.data[i]
			depth = append(depth, int64(level.Price), int64(level.Quantity))
		}
		return depth
	} else {
		n := len(ob.asks.data)
		if n > limit {
			n = limit
		}
		depth := make([]int64, 0, n*2)
		for i := 0; i < n; i++ {
			level := ob.asks.data[i]
			depth = append(depth, int64(level.Price), int64(level.Quantity))
		}
		return depth
	}
}

func (ob *OrderBook) AvailableQuantity(side int8, maxPrice int64, needed types.Quantity) types.Quantity {
	var total types.Quantity
	if side == constants.ORDER_SIDE_BUY {
		for _, level := range ob.asks.data {
			if maxPrice > 0 && int64(level.Price) > maxPrice {
				break
			}
			total += level.Quantity
			if total >= needed {
				break
			}
		}
	} else {
		for _, level := range ob.bids.data {
			if maxPrice > 0 && int64(level.Price) < maxPrice {
				break
			}
			total += level.Quantity
			if total >= needed {
				break
			}
		}
	}
	return total
}
