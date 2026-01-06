package orderbook

import (
	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

type OrderBook struct{}

func New() *OrderBook {
	return &OrderBook{}
}

func (ob *OrderBook) AddOrder(ss *state.OrderBookState, order *types.Order) {
	remaining := order.Quantity - order.Filled
	if remaining <= 0 {
		return
	}

	if order.Side == constants.ORDER_SIDE_BUY {
		ob.addBidLevel(ss, order.Price, remaining, order.ID)
	} else {
		ob.addAskLevel(ss, order.Price, remaining, order.ID)
	}
}

func (ob *OrderBook) addBidLevel(ss *state.OrderBookState, price types.Price, qty types.Quantity, orderID types.OrderID) {
	level, ok := ss.BidIndex[price]
	if !ok {
		level = &state.PriceLevel{
			Price:    price,
			Quantity: 0,
			Orders:   state.NewOrderHeap(),
		}
		ss.BidIndex[price] = level
		ob.linkBid(ss, level)
	}
	level.Quantity += qty
	level.Orders.Push(orderID)
}

func (ob *OrderBook) linkBid(ss *state.OrderBookState, level *state.PriceLevel) {
	if ss.BestBid == nil || level.Price > ss.BestBid.Price {
		level.NextBid = ss.BestBid
		if ss.BestBid != nil {
			ss.BestBid.PrevBid = level
		}
		ss.BestBid = level
		return
	}

	current := ss.BestBid
	for current.NextBid != nil && current.NextBid.Price > level.Price {
		current = current.NextBid
	}

	level.NextBid = current.NextBid
	level.PrevBid = current
	if current.NextBid != nil {
		current.NextBid.PrevBid = level
	}
	current.NextBid = level
}

func (ob *OrderBook) addAskLevel(ss *state.OrderBookState, price types.Price, qty types.Quantity, orderID types.OrderID) {
	level, ok := ss.AskIndex[price]
	if !ok {
		level = &state.PriceLevel{
			Price:    price,
			Quantity: 0,
			Orders:   state.NewOrderHeap(),
		}
		ss.AskIndex[price] = level
		ob.linkAsk(ss, level)
	}
	level.Quantity += qty
	level.Orders.Push(orderID)
}

func (ob *OrderBook) linkAsk(ss *state.OrderBookState, level *state.PriceLevel) {
	if ss.BestAsk == nil || level.Price < ss.BestAsk.Price {
		level.NextAsk = ss.BestAsk
		if ss.BestAsk != nil {
			ss.BestAsk.PrevAsk = level
		}
		ss.BestAsk = level
		return
	}

	current := ss.BestAsk
	for current.NextAsk != nil && current.NextAsk.Price < level.Price {
		current = current.NextAsk
	}

	level.NextAsk = current.NextAsk
	level.PrevAsk = current
	if current.NextAsk != nil {
		current.NextAsk.PrevAsk = level
	}
	current.NextAsk = level
}

func (ob *OrderBook) RemoveOrder(ss *state.OrderBookState, order *types.Order) {
	remaining := order.Quantity - order.Filled
	if remaining <= 0 {
		return
	}

	if order.Side == constants.ORDER_SIDE_BUY {
		ob.removeBidLevel(ss, order.Price, remaining, order.ID)
	} else {
		ob.removeAskLevel(ss, order.Price, remaining, order.ID)
	}
}

func (ob *OrderBook) removeBidLevel(ss *state.OrderBookState, price types.Price, qty types.Quantity, orderID types.OrderID) {
	level, ok := ss.BidIndex[price]
	if !ok {
		return
	}
	level.Quantity -= qty
	level.Orders.Remove(orderID)

	if level.Orders.Len() == 0 && level.Quantity == 0 {
		ob.unlinkBid(ss, level)
		delete(ss.BidIndex, price)
	}
}

func (ob *OrderBook) unlinkBid(ss *state.OrderBookState, level *state.PriceLevel) {
	if level.PrevBid != nil {
		level.PrevBid.NextBid = level.NextBid
	} else {
		ss.BestBid = level.NextBid
	}
	if level.NextBid != nil {
		level.NextBid.PrevBid = level.PrevBid
	}
	level.PrevBid = nil
	level.NextBid = nil
}

func (ob *OrderBook) removeAskLevel(ss *state.OrderBookState, price types.Price, qty types.Quantity, orderID types.OrderID) {
	level, ok := ss.AskIndex[price]
	if !ok {
		return
	}
	level.Quantity -= qty
	level.Orders.Remove(orderID)

	if level.Orders.Len() == 0 && level.Quantity == 0 {
		ob.unlinkAsk(ss, level)
		delete(ss.AskIndex, price)
	}
}

func (ob *OrderBook) unlinkAsk(ss *state.OrderBookState, level *state.PriceLevel) {
	if level.PrevAsk != nil {
		level.PrevAsk.NextAsk = level.NextAsk
	} else {
		ss.BestAsk = level.NextAsk
	}
	if level.NextAsk != nil {
		level.NextAsk.PrevAsk = level.PrevAsk
	}
	level.PrevAsk = nil
	level.NextAsk = nil
}

func (ob *OrderBook) WouldCross(ss *state.OrderBookState, order *types.Order) bool {
	if order.Side == constants.ORDER_SIDE_BUY {
		if ss.BestAsk == nil {
			return false
		}
		return order.Price >= ss.BestAsk.Price
	} else {
		if ss.BestBid == nil {
			return false
		}
		return order.Price <= ss.BestBid.Price
	}
}

func (ob *OrderBook) GetBestBid(ss *state.OrderBookState) types.Price {
	if ss.BestBid == nil {
		return 0
	}
	return ss.BestBid.Price
}

func (ob *OrderBook) GetBestAsk(ss *state.OrderBookState) types.Price {
	if ss.BestAsk == nil {
		return 0
	}
	return ss.BestAsk.Price
}

func (ob *OrderBook) GetDepth(ss *state.OrderBookState, side int8, limit int) []int64 {
	if side == constants.ORDER_SIDE_BUY {
		return ob.getDepthBid(ss.BestBid, limit)
	}
	return ob.getDepthAsk(ss.BestAsk, limit)
}

func (ob *OrderBook) getDepthBid(level *state.PriceLevel, limit int) []int64 {
	if level == nil {
		return nil
	}

	depth := make([]int64, 0, limit*2)

	current := level
	for current != nil && len(depth) < limit*2 {
		depth = append(depth, int64(current.Price), int64(current.Quantity))
		current = current.NextBid
	}

	return depth
}

func (ob *OrderBook) getDepthAsk(level *state.PriceLevel, limit int) []int64 {
	if level == nil {
		return nil
	}

	depth := make([]int64, 0, limit*2)

	current := level
	for current != nil && len(depth) < limit*2 {
		depth = append(depth, int64(current.Price), int64(current.Quantity))
		current = current.NextAsk
	}

	return depth
}

func (ob *OrderBook) MarkLevelEmpty(ss *state.OrderBookState, isBid bool, price types.Price) {
	if isBid {
		level, ok := ss.BidIndex[price]
		if ok {
			ob.unlinkBid(ss, level)
			delete(ss.BidIndex, price)
		}
	} else {
		level, ok := ss.AskIndex[price]
		if ok {
			ob.unlinkAsk(ss, level)
			delete(ss.AskIndex, price)
		}
	}
}

func (ob *OrderBook) AvailableQuantity(ss *state.OrderBookState, side int8, maxPrice int64, needed types.Quantity) types.Quantity {
	var total types.Quantity
	if side == constants.ORDER_SIDE_BUY {
		for level := ss.BestAsk; level != nil && total < needed; level = level.NextAsk {
			if maxPrice > 0 && int64(level.Price) > maxPrice {
				break
			}
			total += level.Quantity
		}
	} else {
		for level := ss.BestBid; level != nil && total < needed; level = level.NextBid {
			if maxPrice > 0 && int64(level.Price) < maxPrice {
				break
			}
			total += level.Quantity
		}
	}
	return total
}

func (ob *OrderBook) Compact(ss *state.OrderBookState) {}
