package state

import (
	"github.com/anomalyco/meta-terminal-go/types"
)

type UserState struct {
	Balances  map[string]*types.UserBalance
	Positions map[string]*types.Position
}

func NewUserState() *UserState {
	return &UserState{
		Balances:  make(map[string]*types.UserBalance),
		Positions: make(map[string]*types.Position),
	}
}

type PriceLevel struct {
	Price    types.Price
	Quantity types.Quantity
	Orders   *OrderHeap
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

type PriceHeap struct {
	Data []types.Price
}

func NewPriceHeap() *PriceHeap {
	return &PriceHeap{
		Data: make([]types.Price, 0),
	}
}

func (h *PriceHeap) Push(price types.Price) {
	h.Data = append(h.Data, price)
	h.siftUp(len(h.Data) - 1)
}

func (h *PriceHeap) Pop() types.Price {
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

func (h *PriceHeap) Peek() types.Price {
	if len(h.Data) == 0 {
		return 0
	}
	return h.Data[0]
}

func (h *PriceHeap) Len() int {
	return len(h.Data)
}

func (h *PriceHeap) siftUp(index int) {
	for index > 0 {
		parent := (index - 1) / 2
		if h.Data[parent] <= h.Data[index] {
			break
		}
		h.Data[parent], h.Data[index] = h.Data[index], h.Data[parent]
		index = parent
	}
}

func (h *PriceHeap) siftDown(index int) {
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

type OrderBookState struct {
	Category int8
	Bids     map[types.Price]*PriceLevel
	Asks     map[types.Price]*PriceLevel
	BestBid  *PriceLevel
	BestAsk  *PriceLevel
	BidHeap  *PriceHeap
	AskHeap  *PriceHeap
}

func NewOrderBookState() *OrderBookState {
	return &OrderBookState{
		Bids:    make(map[types.Price]*PriceLevel),
		Asks:    make(map[types.Price]*PriceLevel),
		BidHeap: NewPriceHeap(),
		AskHeap: NewPriceHeap(),
	}
}

type SymbolContainer struct {
	Symbol   string
	Category int8

	SpotOrderBook   *OrderBookState
	LinearOrderBook *OrderBookState

	LinearTriggers *TriggerHeap
}

type TriggerHeap struct {
	BuyHeap  []types.OrderID
	SellHeap []types.OrderID
}

func NewTriggerHeap() *TriggerHeap {
	return &TriggerHeap{
		BuyHeap:  make([]types.OrderID, 0),
		SellHeap: make([]types.OrderID, 0),
	}
}

type EngineState struct {
	Users      map[types.UserID]*UserState
	Containers map[string]*SymbolContainer
}

func NewEngineState() *EngineState {
	return &EngineState{
		Users:      make(map[types.UserID]*UserState),
		Containers: make(map[string]*SymbolContainer),
	}
}

func (s *EngineState) GetUserState(userID types.UserID) *UserState {
	us, ok := s.Users[userID]
	if !ok {
		us = NewUserState()
		s.Users[userID] = us
	}
	return us
}

func (s *EngineState) GetContainer(symbol string) *SymbolContainer {
	return s.Containers[symbol]
}

func (s *EngineState) RegisterContainer(container *SymbolContainer) {
	s.Containers[container.Symbol] = container
}
