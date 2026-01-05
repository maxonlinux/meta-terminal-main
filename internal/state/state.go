package state

import (
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

type State struct {
	Users       map[types.UserID]*UserState
	Symbols     map[types.SymbolID]*SymbolState
	OrderByID   map[types.OrderID]*types.Order
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
	OrderMap       map[types.OrderID]*types.Order
	BuyTriggers    *Heap
	SellTriggers   *Heap
	UserReduceOnly map[types.UserID][]types.OrderID
}

type PriceLevel struct {
	Price          types.Price
	Quantity       types.Quantity
	Orders         map[types.OrderID]*types.Order
	PrevPriceLevel *PriceLevel
	NextPriceLevel *PriceLevel
}

func New() *State {
	return &State{
		Users:       make(map[types.UserID]*UserState),
		Symbols:     make(map[types.SymbolID]*SymbolState),
		OrderByID:   make(map[types.OrderID]*types.Order),
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
		OrderMap:       make(map[types.OrderID]*types.Order),
		BuyTriggers:    nil,
		SellTriggers:   nil,
		UserReduceOnly: make(map[types.UserID][]types.OrderID),
	}
	s.Symbols[symbol] = ss
	return ss
}

func (s *SymbolState) AddReduceOnlyOrder(userID types.UserID, orderID types.OrderID) {
	s.UserReduceOnly[userID] = append(s.UserReduceOnly[userID], orderID)
}

func (s *SymbolState) RemoveReduceOnlyOrder(userID types.UserID, orderID types.OrderID) {
	oids := s.UserReduceOnly[userID]
	for i, oid := range oids {
		if oid == orderID {
			s.UserReduceOnly[userID] = append(oids[:i], oids[i+1:]...)
			break
		}
	}
}

func (s *SymbolState) GetUserReduceOnlyOrders(userID types.UserID) []types.OrderID {
	return s.UserReduceOnly[userID]
}
