package linked_list

import (
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
	NextBid  *PriceLevel
	PrevBid  *PriceLevel
	NextAsk  *PriceLevel
	PrevAsk  *PriceLevel
}

type OrderBookState struct {
	BidIndex map[types.Price]*PriceLevel
	AskIndex map[types.Price]*PriceLevel
}

func NewState() *OrderBookState {
	return &OrderBookState{
		BidIndex: make(map[types.Price]*PriceLevel),
		AskIndex: make(map[types.Price]*PriceLevel),
	}
}

type OrderBook struct {
	BestBid *PriceLevel
	BestAsk *PriceLevel
}

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
		ob.linkBid(level)
	}
	level.Quantity += qty
	level.Orders.Push(orderID)
}

func (ob *OrderBook) linkBid(level *PriceLevel) {
	if ob.BestBid == nil || level.Price > ob.BestBid.Price {
		level.NextBid = ob.BestBid
		if ob.BestBid != nil {
			ob.BestBid.PrevBid = level
		}
		ob.BestBid = level
		return
	}

	current := ob.BestBid
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

func (ob *OrderBook) AddAsk(ss *OrderBookState, price types.Price, qty types.Quantity, orderID types.OrderID) {
	level, ok := ss.AskIndex[price]
	if !ok {
		level = &PriceLevel{
			Price:    price,
			Quantity: 0,
			Orders:   NewOrderHeap(),
		}
		ss.AskIndex[price] = level
		ob.linkAsk(level)
	}
	level.Quantity += qty
	level.Orders.Push(orderID)
}

func (ob *OrderBook) linkAsk(level *PriceLevel) {
	if ob.BestAsk == nil || level.Price < ob.BestAsk.Price {
		level.NextAsk = ob.BestAsk
		if ob.BestAsk != nil {
			ob.BestAsk.PrevAsk = level
		}
		ob.BestAsk = level
		return
	}

	current := ob.BestAsk
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

func (ob *OrderBook) RemoveOrder(ss *OrderBookState, price types.Price, qty types.Quantity, orderID types.OrderID) {
	level, ok := ss.BidIndex[price]
	if !ok {
		return
	}
	level.Quantity -= qty
	level.Orders.Remove(orderID)

	if level.Orders.Len() == 0 && level.Quantity == 0 {
		ob.unlinkBid(level)
		delete(ss.BidIndex, price)
	}
}

func (ob *OrderBook) unlinkBid(level *PriceLevel) {
	if level.PrevBid != nil {
		level.PrevBid.NextBid = level.NextBid
	} else {
		ob.BestBid = level.NextBid
	}
	if level.NextBid != nil {
		level.NextBid.PrevBid = level.PrevBid
	}
	level.PrevBid = nil
	level.NextBid = nil
}

func (ob *OrderBook) RemoveAsk(ss *OrderBookState, price types.Price, qty types.Quantity, orderID types.OrderID) {
	level, ok := ss.AskIndex[price]
	if !ok {
		return
	}
	level.Quantity -= qty
	level.Orders.Remove(orderID)

	if level.Orders.Len() == 0 && level.Quantity == 0 {
		ob.unlinkAsk(level)
		delete(ss.AskIndex, price)
	}
}

func (ob *OrderBook) unlinkAsk(level *PriceLevel) {
	if level.PrevAsk != nil {
		level.PrevAsk.NextAsk = level.NextAsk
	} else {
		ob.BestAsk = level.NextAsk
	}
	if level.NextAsk != nil {
		level.NextAsk.PrevAsk = level.PrevAsk
	}
	level.PrevAsk = nil
	level.NextAsk = nil
}

func (ob *OrderBook) WouldCross(price types.Price, ss *OrderBookState) bool {
	if ob.BestAsk == nil {
		return false
	}
	return price >= ob.BestAsk.Price
}

func (ob *OrderBook) WouldCrossAsk(price types.Price, ss *OrderBookState) bool {
	if ob.BestBid == nil {
		return false
	}
	return price <= ob.BestBid.Price
}

func (ob *OrderBook) GetBestBid(ss *OrderBookState) types.Price {
	if ob.BestBid == nil {
		return 0
	}
	return ob.BestBid.Price
}

func (ob *OrderBook) GetBestAsk(ss *OrderBookState) types.Price {
	if ob.BestAsk == nil {
		return 0
	}
	return ob.BestAsk.Price
}

func (ob *OrderBook) GetDepth(ss *OrderBookState, limit int) []int64 {
	depth := make([]int64, 0, limit*2)

	current := ob.BestBid
	for current != nil && len(depth) < limit*2 {
		depth = append(depth, int64(current.Price), int64(current.Quantity))
		current = current.NextBid
	}

	return depth
}

func (ob *OrderBook) GetAskDepth(ss *OrderBookState, limit int) []int64 {
	depth := make([]int64, 0, limit*2)

	current := ob.BestAsk
	for current != nil && len(depth) < limit*2 {
		depth = append(depth, int64(current.Price), int64(current.Quantity))
		current = current.NextAsk
	}

	return depth
}

func (ob *OrderBook) MarkLevelEmpty(ss *OrderBookState, isBid bool, price types.Price) {
	if isBid {
		level, ok := ss.BidIndex[price]
		if ok {
			ob.unlinkBid(level)
			delete(ss.BidIndex, price)
		}
	} else {
		level, ok := ss.AskIndex[price]
		if ok {
			ob.unlinkAsk(level)
			delete(ss.AskIndex, price)
		}
	}
}

func (ob *OrderBook) Compact(ss *OrderBookState) {}
