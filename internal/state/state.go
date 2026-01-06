package state

import (
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

type State struct {
	Users       map[types.UserID]*UserState
	Symbols     map[types.SymbolID]*SymbolState
	OrderByID   map[types.OrderID]*types.Order
	UsersOrders map[types.UserID]map[types.OrderID]*types.Order
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
	UserReduceOnly map[types.UserID]map[types.OrderID]struct{}
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
		UsersOrders: make(map[types.UserID]map[types.OrderID]*types.Order),
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
		UserReduceOnly: make(map[types.UserID]map[types.OrderID]struct{}),
	}
	s.Symbols[symbol] = ss
	return ss
}

func (s *State) InitSymbolCategory(symbol types.SymbolID, category int8) {
	ss := s.GetSymbolState(symbol)
	ss.Category = category
}

func (s *SymbolState) AddReduceOnlyOrder(userID types.UserID, orderID types.OrderID) {
	if s.UserReduceOnly[userID] == nil {
		s.UserReduceOnly[userID] = make(map[types.OrderID]struct{})
	}
	s.UserReduceOnly[userID][orderID] = struct{}{}
}

func (s *SymbolState) RemoveReduceOnlyOrder(userID types.UserID, orderID types.OrderID) {
	if s.UserReduceOnly[userID] != nil {
		delete(s.UserReduceOnly[userID], orderID)
	}
}

func (s *SymbolState) GetUserReduceOnlyOrders(userID types.UserID) []types.OrderID {
	if s.UserReduceOnly[userID] == nil {
		return nil
	}
	oids := make([]types.OrderID, 0, len(s.UserReduceOnly[userID]))
	for oid := range s.UserReduceOnly[userID] {
		oids = append(oids, oid)
	}
	return oids
}

func (s *State) AddOrder(order *types.Order) {
	s.OrderByID[order.ID] = order
	if s.UsersOrders[order.UserID] == nil {
		s.UsersOrders[order.UserID] = make(map[types.OrderID]*types.Order)
	}
	s.UsersOrders[order.UserID][order.ID] = order
}

func (s *State) RemoveOrder(orderID types.OrderID) {
	order, ok := s.OrderByID[orderID]
	if !ok {
		return
	}
	delete(s.OrderByID, orderID)
	if s.UsersOrders[order.UserID] != nil {
		delete(s.UsersOrders[order.UserID], orderID)
	}
}
