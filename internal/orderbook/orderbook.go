package orderbook

import (
	"sort"

	"slices"

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
		ob.insertPriceSorted(&ss.BidPrices, price, true)
	}
	level.Quantity += qty
	level.Orders.Push(orderID)
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
		ob.insertPriceSorted(&ss.AskPrices, price, false)
	}
	level.Quantity += qty
	level.Orders.Push(orderID)
}

func (ob *OrderBook) insertPriceSorted(prices *[]types.Price, price types.Price, descending bool) {
	arr := *prices
	i := sort.Search(len(arr), func(j int) bool {
		if descending {
			return arr[j] <= price
		}
		return arr[j] >= price
	})
	if i < len(arr) && arr[i] == price {
		return
	}
	*prices = append(arr[:i], append([]types.Price{price}, arr[i:]...)...)
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
		delete(ss.BidIndex, price)
		ob.RemovePrice(&ss.BidPrices, price)
	}
}

func (ob *OrderBook) removeAskLevel(ss *state.OrderBookState, price types.Price, qty types.Quantity, orderID types.OrderID) {
	level, ok := ss.AskIndex[price]
	if !ok {
		return
	}
	level.Quantity -= qty
	level.Orders.Remove(orderID)

	if level.Orders.Len() == 0 && level.Quantity == 0 {
		delete(ss.AskIndex, price)
		ob.RemovePrice(&ss.AskPrices, price)
	}
}

func (ob *OrderBook) RemovePrice(prices *[]types.Price, price types.Price) {
	arr := *prices
	i := sort.Search(len(arr), func(j int) bool {
		return arr[j] >= price
	})
	if i < len(arr) && arr[i] == price {
		*prices = slices.Delete(arr, i, i+1)
	}
}

func (ob *OrderBook) WouldCross(ss *state.OrderBookState, order *types.Order) bool {
	if order.Side == constants.ORDER_SIDE_BUY {
		if len(ss.AskPrices) == 0 {
			return false
		}
		return order.Price >= ss.AskPrices[0]
	} else {
		if len(ss.BidPrices) == 0 {
			return false
		}
		return order.Price <= ss.BidPrices[0]
	}
}

func (ob *OrderBook) GetBestBid(ss *state.OrderBookState) types.Price {
	if len(ss.BidPrices) == 0 {
		return 0
	}
	return ss.BidPrices[0]
}

func (ob *OrderBook) GetBestAsk(ss *state.OrderBookState) types.Price {
	if len(ss.AskPrices) == 0 {
		return 0
	}
	return ss.AskPrices[0]
}

func (ob *OrderBook) GetDepth(ss *state.OrderBookState, side int8, limit int) []int64 {
	if side == constants.ORDER_SIDE_BUY {
		return ob.getDepthFlat(ss.BidPrices, ss.BidIndex, limit)
	}
	return ob.getDepthFlat(ss.AskPrices, ss.AskIndex, limit)
}

func (ob *OrderBook) getDepthFlat(prices []types.Price, index map[types.Price]*state.PriceLevel, limit int) []int64 {
	if len(prices) == 0 {
		return nil
	}

	count := min(len(prices), limit)

	depth := make([]int64, count*2)

	for i := range count {
		price := prices[i]
		level := index[price]
		if level == nil {
			depth[i*2] = int64(price)
			depth[i*2+1] = 0
		} else {
			depth[i*2] = int64(price)
			depth[i*2+1] = int64(level.Quantity)
		}
	}

	return depth
}
