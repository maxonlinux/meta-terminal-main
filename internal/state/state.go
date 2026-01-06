package state

import (
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

type State struct {
	Users       map[types.UserID]*UserState
	Symbols     map[types.SymbolID]*SymbolState
	NextOrderID types.OrderID
}

type UserState struct {
	Balances  map[string]*types.UserBalance
	Positions map[types.SymbolID]*types.Position
}

type SymbolState struct {
	Category       int8
	Bids           *PriceLevel
	Asks           *PriceLevel
	BidLevels      map[types.Price]*PriceLevel
	AskLevels      map[types.Price]*PriceLevel
	OrderMap       map[types.OrderID]*types.Order
	BuyTriggers    *Heap
	SellTriggers   *Heap
	UserReduceOnly map[types.UserID]map[types.OrderID]struct{}
}

type PriceLevel struct {
	Price          types.Price
	Quantity       types.Quantity
	Orders         *OrderHeap
	FirstOrderID   types.OrderID
	PrevPriceLevel *PriceLevel
	NextPriceLevel *PriceLevel
}

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
	h.siftUp(len(h.Data) - 1)
}

func (h *OrderHeap) Pop() types.OrderID {
	if len(h.Data) == 0 {
		return 0
	}

	min := h.Data[0]
	h.Data[0] = h.Data[len(h.Data)-1]
	h.Data = h.Data[:len(h.Data)-1]
	if len(h.Data) > 0 {
		h.siftDown(0)
	}
	return min
}

func (h *OrderHeap) Peek() types.OrderID {
	if len(h.Data) == 0 {
		return 0
	}
	return h.Data[0]
}

func (h *OrderHeap) Len() int {
	return len(h.Data)
}

func (h *OrderHeap) siftUp(index int) {
	for index > 0 {
		parent := (index - 1) / 2
		if h.Data[parent] <= h.Data[index] {
			break
		}
		h.Data[parent], h.Data[index] = h.Data[index], h.Data[parent]
		index = parent
	}
}

func (h *OrderHeap) siftDown(index int) {
	n := len(h.Data)
	for index < n {
		left := 2*index + 1
		right := left + 1
		smallest := index

		if left < n && h.Data[left] < h.Data[smallest] {
			smallest = left
		}
		if right < n && h.Data[right] < h.Data[smallest] {
			smallest = right
		}
		if smallest == index {
			break
		}
		h.Data[index], h.Data[smallest] = h.Data[smallest], h.Data[index]
		index = smallest
	}
}

func (h *OrderHeap) Remove(orderID types.OrderID) bool {
	for i, id := range h.Data {
		if id == orderID {
			h.Data = append(h.Data[:i], h.Data[i+1:]...)
			h.siftDown(i)
			return true
		}
	}
	return false
}

func New() *State {
	return &State{
		Users:       make(map[types.UserID]*UserState),
		Symbols:     make(map[types.SymbolID]*SymbolState),
		NextOrderID: 1,
	}
}

func (s *State) GetUserState(userID types.UserID) *UserState {
	if us, ok := s.Users[userID]; ok {
		return us
	}
	us := &UserState{
		Balances:  make(map[string]*types.UserBalance),
		Positions: make(map[types.SymbolID]*types.Position),
	}
	s.Users[userID] = us
	return us
}

func (s *State) GetSymbolState(symbol types.SymbolID) *SymbolState {
	if ss, ok := s.Symbols[symbol]; ok {
		return ss
	}
	ss := &SymbolState{
		Category:       0,
		Bids:           nil,
		Asks:           nil,
		BidLevels:      make(map[types.Price]*PriceLevel),
		AskLevels:      make(map[types.Price]*PriceLevel),
		OrderMap:       make(map[types.OrderID]*types.Order),
		BuyTriggers:    nil,
		SellTriggers:   nil,
		UserReduceOnly: make(map[types.UserID]map[types.OrderID]struct{}),
	}
	s.Symbols[symbol] = ss
	return ss
}

func (ss *SymbolState) AddBidLevel(price types.Price) *PriceLevel {
	if level, ok := ss.BidLevels[price]; ok {
		return level
	}
	level := &PriceLevel{
		Price:  price,
		Orders: NewOrderHeap(),
	}
	ss.BidLevels[price] = level
	level.NextPriceLevel = ss.Bids
	ss.Bids = level
	return level
}

func (ss *SymbolState) AddAskLevel(price types.Price) *PriceLevel {
	if level, ok := ss.AskLevels[price]; ok {
		return level
	}
	level := &PriceLevel{
		Price:  price,
		Orders: NewOrderHeap(),
	}
	ss.AskLevels[price] = level
	level.NextPriceLevel = ss.Asks
	ss.Asks = level
	return level
}

func (ss *SymbolState) GetUserReduceOnlyOrders(userID types.UserID) []types.OrderID {
	if orders, ok := ss.UserReduceOnly[userID]; ok {
		result := make([]types.OrderID, 0, len(orders))
		for oid := range orders {
			result = append(result, oid)
		}
		return result
	}
	return nil
}

func (ss *SymbolState) RemoveReduceOnlyOrder(userID types.UserID, orderID types.OrderID) {
	if orders, ok := ss.UserReduceOnly[userID]; ok {
		delete(orders, orderID)
	}
}
