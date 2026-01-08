package orderstore

import (
	"sync"

	"github.com/anomalyco/meta-terminal-go/internal/types"
)

type Store struct {
	mu     sync.RWMutex
	byUser map[types.UserID]map[types.OrderID]*types.Order
	byID   map[types.OrderID]*types.Order
	ro     map[types.UserID]map[types.OrderID]struct{}
}

func New() *Store {
	return &Store{
		byUser: make(map[types.UserID]map[types.OrderID]*types.Order),
		byID:   make(map[types.OrderID]*types.Order),
		ro:     make(map[types.UserID]map[types.OrderID]struct{}),
	}
}

func (s *Store) Add(order *types.Order) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.byUser[order.UserID] == nil {
		s.byUser[order.UserID] = make(map[types.OrderID]*types.Order)
	}
	s.byUser[order.UserID][order.ID] = order
	s.byID[order.ID] = order
	if order.ReduceOnly {
		if s.ro[order.UserID] == nil {
			s.ro[order.UserID] = make(map[types.OrderID]struct{})
		}
		s.ro[order.UserID][order.ID] = struct{}{}
	}
}

func (s *Store) Get(userID types.UserID, orderID types.OrderID) *types.Order {
	s.mu.RLock()
	defer s.mu.RUnlock()

	userOrders := s.byUser[userID]
	if userOrders == nil {
		return nil
	}
	return userOrders[orderID]
}

func (s *Store) GetByID(orderID types.OrderID) *types.Order {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.byID[orderID]
}

func (s *Store) Remove(userID types.UserID, orderID types.OrderID) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.byUser[userID] != nil {
		delete(s.byUser[userID], orderID)
		if len(s.byUser[userID]) == 0 {
			delete(s.byUser, userID)
		}
	}
	delete(s.byID, orderID)
	if s.ro[userID] != nil {
		delete(s.ro[userID], orderID)
		if len(s.ro[userID]) == 0 {
			delete(s.ro, userID)
		}
	}
}

func (s *Store) UserOrders(userID types.UserID) []*types.Order {
	s.mu.RLock()
	defer s.mu.RUnlock()

	userOrders := s.byUser[userID]
	if userOrders == nil {
		return nil
	}

	orders := make([]*types.Order, 0, len(userOrders))
	for _, order := range userOrders {
		orders = append(orders, order)
	}
	return orders
}

func (s *Store) UserReduceOnlyOrders(userID types.UserID) []types.OrderID {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.ro[userID] == nil {
		return nil
	}
	out := make([]types.OrderID, 0, len(s.ro[userID]))
	for id := range s.ro[userID] {
		out = append(out, id)
	}
	return out
}

func (s *Store) All() []*types.Order {
	s.mu.RLock()
	defer s.mu.RUnlock()

	orders := make([]*types.Order, 0, len(s.byID))
	for _, order := range s.byID {
		orders = append(orders, order)
	}
	return orders
}

func (s *Store) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byUser = make(map[types.UserID]map[types.OrderID]*types.Order)
	s.byID = make(map[types.OrderID]*types.Order)
	s.ro = make(map[types.UserID]map[types.OrderID]struct{})
}
