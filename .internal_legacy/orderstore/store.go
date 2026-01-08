package orderstore

import (
	"sync"

	"github.com/anomalyco/meta-terminal-go/internal/domain"
)

type Store struct {
	mu     sync.RWMutex
	byUser map[domain.UserID]map[domain.OrderID]*domain.Order
	byID   map[domain.OrderID]*domain.Order
}

func New() *Store {
	return &Store{
		byUser: make(map[domain.UserID]map[domain.OrderID]*domain.Order),
		byID:   make(map[domain.OrderID]*domain.Order),
	}
}

func (s *Store) Add(order *domain.Order) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.byUser[order.UserID] == nil {
		s.byUser[order.UserID] = make(map[domain.OrderID]*domain.Order)
	}
	s.byUser[order.UserID][order.ID] = order
	s.byID[order.ID] = order
}

func (s *Store) Get(userID domain.UserID, orderID domain.OrderID) *domain.Order {
	s.mu.RLock()
	defer s.mu.RUnlock()

	userOrders := s.byUser[userID]
	if userOrders == nil {
		return nil
	}
	return userOrders[orderID]
}

func (s *Store) GetByID(orderID domain.OrderID) *domain.Order {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.byID[orderID]
}

func (s *Store) Remove(userID domain.UserID, orderID domain.OrderID) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.byUser[userID] != nil {
		delete(s.byUser[userID], orderID)
		if len(s.byUser[userID]) == 0 {
			delete(s.byUser, userID)
		}
	}
	delete(s.byID, orderID)
}

func (s *Store) UserOrderIDs(userID domain.UserID) []domain.OrderID {
	s.mu.RLock()
	defer s.mu.RUnlock()

	userOrders := s.byUser[userID]
	if userOrders == nil {
		return nil
	}

	ids := make([]domain.OrderID, 0, len(userOrders))
	for id := range userOrders {
		ids = append(ids, id)
	}
	return ids
}
