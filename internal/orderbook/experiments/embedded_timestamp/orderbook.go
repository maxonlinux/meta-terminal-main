package embedded_timestamp

import (
	"sort"
	"time"

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
	Price     types.Price
	Quantity  types.Quantity
	Orders    *OrderHeap
	CleanedAt time.Time
}

type OrderBookState struct {
	BidIndex    map[types.Price]*PriceLevel
	BidPrices   []types.Price
	AskIndex    map[types.Price]*PriceLevel
	AskPrices   []types.Price
	LastCompact time.Time
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
		level.CleanedAt = time.Now()
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
		level.CleanedAt = time.Now()
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
	cutoff := ss.LastCompact
	for _, price := range ss.BidPrices {
		level := ss.BidIndex[price]
		if level != nil && (level.CleanedAt.IsZero() || level.CleanedAt.After(cutoff)) {
			return price
		}
	}
	return 0
}

func (ob *OrderBook) GetBestAsk(ss *OrderBookState) types.Price {
	cutoff := ss.LastCompact
	for _, price := range ss.AskPrices {
		level := ss.AskIndex[price]
		if level != nil && (level.CleanedAt.IsZero() || level.CleanedAt.After(cutoff)) {
			return price
		}
	}
	return 0
}

func (ob *OrderBook) GetDepth(ss *OrderBookState, limit int) []int64 {
	cutoff := ss.LastCompact
	count := 0
	depth := make([]int64, limit*2)

	for _, price := range ss.BidPrices {
		if count >= limit {
			break
		}
		level := ss.BidIndex[price]
		if level == nil {
			continue
		}
		if !level.CleanedAt.IsZero() && !level.CleanedAt.After(cutoff) {
			continue
		}
		depth[count*2] = int64(price)
		depth[count*2+1] = int64(level.Quantity)
		count++
	}

	return depth[:count*2]
}

func (ob *OrderBook) GetAskDepth(ss *OrderBookState, limit int) []int64 {
	cutoff := ss.LastCompact
	count := 0
	depth := make([]int64, limit*2)

	for _, price := range ss.AskPrices {
		if count >= limit {
			break
		}
		level := ss.AskIndex[price]
		if level == nil {
			continue
		}
		if !level.CleanedAt.IsZero() && !level.CleanedAt.After(cutoff) {
			continue
		}
		depth[count*2] = int64(price)
		depth[count*2+1] = int64(level.Quantity)
		count++
	}

	return depth[:count*2]
}

func (ob *OrderBook) MarkLevelEmpty(ss *OrderBookState, isBid bool, price types.Price) {
	if isBid {
		level, ok := ss.BidIndex[price]
		if ok {
			level.CleanedAt = time.Now()
		}
	} else {
		level, ok := ss.AskIndex[price]
		if ok {
			level.CleanedAt = time.Now()
		}
	}
}

func (ob *OrderBook) Compact(ss *OrderBookState) {
	now := time.Now()
	ss.LastCompact = now
}
