package dead_stack

import (
	"slices"
	"sort"

	"github.com/anomalyco/meta-terminal-go/internal/types"
)

type OrderHeap struct {
	Data []types.OrderID
}

func NewOrderHeap() *OrderHeap {
	return &OrderHeap{
		Data: make([]types.OrderID, 0),
	}
}

func (h *OrderHeap) Push(orderID types.OrderID) {
	h.Data = append(h.Data, orderID)
}

func (h *OrderHeap) Pop() types.OrderID {
	if len(h.Data) == 0 {
		return 0
	}
	id := h.Data[0]
	h.Data = h.Data[1:]
	return id
}

func (h *OrderHeap) Peek() types.OrderID {
	if len(h.Data) == 0 {
		return 0
	}
	return h.Data[0]
}

func (h *OrderHeap) Remove(orderID types.OrderID) {
	for i, id := range h.Data {
		if id == orderID {
			h.Data = append(h.Data[:i], h.Data[i+1:]...)
			return
		}
	}
}

func (h *OrderHeap) Len() int {
	return len(h.Data)
}

type PriceLevel struct {
	Price    types.Price
	Quantity types.Quantity
	Orders   *OrderHeap
}

type OrderBookState struct {
	BidIndex      map[types.Price]*PriceLevel
	BidPrices     []types.Price
	DeadBidPrices []types.Price
	AskIndex      map[types.Price]*PriceLevel
	AskPrices     []types.Price
	DeadAskPrices []types.Price
}

func NewState() *OrderBookState {
	return &OrderBookState{
		BidIndex: make(map[types.Price]*PriceLevel),
		AskIndex: make(map[types.Price]*PriceLevel),
	}
}

type OrderBook struct{}

func New() *OrderBook {
	return &OrderBook{}
}

func (ob *OrderBook) AddOrder(ss *OrderBookState, price types.Price, qty types.Quantity, orderID types.OrderID) {
	level, ok := ss.BidIndex[price]
	if !ok {
		level = &PriceLevel{
			Price:    price,
			Quantity: 0,
			Orders:   NewOrderHeap(),
		}
		ss.BidIndex[price] = level
		ob.insertPriceSorted(&ss.BidPrices, price, true)
	}
	level.Quantity += qty
	level.Orders.Push(orderID)
}

func (ob *OrderBook) AddAsk(ss *OrderBookState, price types.Price, qty types.Quantity, orderID types.OrderID) {
	level, ok := ss.AskIndex[price]
	if !ok {
		level = &PriceLevel{
			Price:    price,
			Quantity: 0,
			Orders:   NewOrderHeap(),
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

func (ob *OrderBook) RemoveOrder(ss *OrderBookState, price types.Price, qty types.Quantity, orderID types.OrderID) {
	level, ok := ss.BidIndex[price]
	if !ok {
		return
	}
	level.Quantity -= qty
	level.Orders.Remove(orderID)

	if level.Orders.Len() == 0 && level.Quantity == 0 {
		ss.DeadBidPrices = append(ss.DeadBidPrices, price)
	}
}

func (ob *OrderBook) RemoveAsk(ss *OrderBookState, price types.Price, qty types.Quantity, orderID types.OrderID) {
	level, ok := ss.AskIndex[price]
	if !ok {
		return
	}
	level.Quantity -= qty
	level.Orders.Remove(orderID)

	if level.Orders.Len() == 0 && level.Quantity == 0 {
		ss.DeadAskPrices = append(ss.DeadAskPrices, price)
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

func (ob *OrderBook) WouldCross(price types.Price, ss *OrderBookState) bool {
	if len(ss.AskPrices) == 0 {
		return false
	}
	return price >= ss.AskPrices[0]
}

func (ob *OrderBook) WouldCrossAsk(price types.Price, ss *OrderBookState) bool {
	if len(ss.BidPrices) == 0 {
		return false
	}
	return price <= ss.BidPrices[0]
}

func (ob *OrderBook) GetBestBid(ss *OrderBookState) types.Price {
	if len(ss.BidPrices) == 0 {
		return 0
	}
	return ss.BidPrices[0]
}

func (ob *OrderBook) GetBestAsk(ss *OrderBookState) types.Price {
	if len(ss.AskPrices) == 0 {
		return 0
	}
	return ss.AskPrices[0]
}

func (ob *OrderBook) GetDepth(ss *OrderBookState, limit int) []int64 {
	if len(ss.BidPrices) == 0 {
		return nil
	}

	count := min(len(ss.BidPrices), limit)
	depth := make([]int64, count*2)

	for i := range count {
		price := ss.BidPrices[i]
		level := ss.BidIndex[price]
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

func (ob *OrderBook) GetAskDepth(ss *OrderBookState, limit int) []int64 {
	if len(ss.AskPrices) == 0 {
		return nil
	}

	count := min(len(ss.AskPrices), limit)
	depth := make([]int64, count*2)

	for i := range count {
		price := ss.AskPrices[i]
		level := ss.AskIndex[price]
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

func (ob *OrderBook) MarkLevelEmpty(ss *OrderBookState, isBid bool, price types.Price) {
	if isBid {
		ss.DeadBidPrices = append(ss.DeadBidPrices, price)
	} else {
		ss.DeadAskPrices = append(ss.DeadAskPrices, price)
	}
}

func (ob *OrderBook) Compact(ss *OrderBookState) {
	for _, price := range ss.DeadBidPrices {
		delete(ss.BidIndex, price)
		ob.RemovePrice(&ss.BidPrices, price)
	}
	ss.DeadBidPrices = ss.DeadBidPrices[:0]

	for _, price := range ss.DeadAskPrices {
		delete(ss.AskIndex, price)
		ob.RemovePrice(&ss.AskPrices, price)
	}
	ss.DeadAskPrices = ss.DeadAskPrices[:0]
}
