package oms

import (
	"sync"

	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/snowflake"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

// OrderStore — единственный источник истины по ордерам во всей системе.
// Все остальные компоненты хранят только ссылки на ордера отсюда.
type OrderStore struct {
	mu        sync.RWMutex
	allOrders map[types.OrderID]*types.Order
}

func NewOrderStore() *OrderStore {
	return &OrderStore{
		allOrders: make(map[types.OrderID]*types.Order),
	}
}

// Create создает новый ордер
// Единственный способ создания ордеров в системе
func (s *OrderStore) Create(
	userID types.UserID,
	symbol string,
	category int8,
	side int8,
	otype int8,
	tif int8,
	price types.Price,
	quantity types.Quantity,
	triggerPrice types.Price,
	reduceOnly bool,
	closeOnTrigger bool,
	stopOrderType int8,
) *types.Order {
	s.mu.Lock()
	defer s.mu.Unlock()

	order := &types.Order{
		ID:             types.OrderID(snowflake.Next()),
		UserID:         userID,
		Symbol:         symbol,
		Category:       category,
		Side:           side,
		Type:           otype,
		TIF:            tif,
		Status:         constants.ORDER_STATUS_NEW,
		Price:          price,
		Quantity:       quantity,
		Filled:         0,
		TriggerPrice:   triggerPrice,
		ReduceOnly:     reduceOnly,
		CloseOnTrigger: closeOnTrigger,
		StopOrderType:  stopOrderType,
		IsConditional:  triggerPrice > 0,
		CreatedAt:      types.NowNano(),
		UpdatedAt:      types.NowNano(),
	}

	s.allOrders[order.ID] = order
	return order
}

// Get возвращает ордер по ID
func (s *OrderStore) Get(id types.OrderID) (*types.Order, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	order, ok := s.allOrders[id]
	return order, ok
}

// Count возвращает количество ордеров
func (s *OrderStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.allOrders)
}
