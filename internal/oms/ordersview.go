package oms

import (
	"context"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/orderbook"
	"github.com/anomalyco/meta-terminal-go/internal/pool"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func (s *Service) GetOrder(userID types.UserID, orderID types.OrderID) *types.Order {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if userOrders := s.orders[userID]; userOrders != nil {
		return userOrders[orderID]
	}
	return nil
}

func (s *Service) GetOrders(userID types.UserID) []*types.Order {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if userOrders := s.orders[userID]; userOrders != nil {
		orders := make([]*types.Order, 0, len(userOrders))
		for _, order := range userOrders {
			orders = append(orders, order)
		}
		return orders
	}
	return nil
}

func (s *Service) CancelOrder(ctx context.Context, userID types.UserID, orderID types.OrderID) error {
	userOrders := s.orders[userID]
	if userOrders == nil {
		return nil
	}

	order := userOrders[orderID]
	if order == nil {
		return nil
	}

	var ob *orderbook.OrderBook
	if order.Status == constants.ORDER_STATUS_UNTRIGGERED {
		s.triggerMon.Remove(orderID)
	} else {
		ob = s.getOrderBookIfExists(order.Category, order.Symbol)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if ob != nil {
		ob.Remove(order.ID)
	}
	delete(userOrders, orderID)
	delete(s.ordersByID, orderID)
	if len(userOrders) == 0 {
		delete(s.orders, userID)
	}

	remaining := order.Quantity - order.Filled
	if remaining > 0 && order.Status != constants.ORDER_STATUS_UNTRIGGERED {
		s.clearing.Release(order.UserID, order.Symbol, order.Category, order.Side, remaining, order.Price)
	}

	if order.Filled > 0 {
		order.Status = constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED
	} else {
		order.Status = constants.ORDER_STATUS_CANCELED
	}
	order.UpdatedAt = types.NowNano()

	s.publishOrderEvent(order)

	pool.PutOrder(order)

	return nil
}
