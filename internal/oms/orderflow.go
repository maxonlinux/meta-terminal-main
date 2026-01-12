package oms

import (
	"context"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/pool"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func (s *Service) PlaceOrder(ctx context.Context, input *types.OrderInput) (*types.OrderResult, error) {
	if err := s.validateOrder(input); err != nil {
		return nil, err
	}

	if err := s.checkSelfMatch(input); err != nil {
		return nil, err
	}

	if input.TIF == constants.TIF_FOK {
		ob := s.getOrderBook(input.Category, input.Symbol)
		var limitPrice types.Price
		if input.Type == constants.ORDER_TYPE_LIMIT {
			limitPrice = input.Price
		}
		available := ob.AvailableQuantity(input.Side, limitPrice, input.Quantity)
		if available < input.Quantity {
			return nil, ErrFOKInsufficientLiquidity
		}
	}

	if input.OCO != nil {
		return s.placeOCOOrder(ctx, input)
	}

	order := pool.GetOrder()
	order.ID = types.OrderID(poolGetOrderID())
	order.UserID = input.UserID
	order.Symbol = input.Symbol
	order.Category = input.Category
	order.Side = input.Side
	order.Type = input.Type
	order.TIF = input.TIF
	order.Status = constants.ORDER_STATUS_NEW
	order.Price = input.Price
	order.Quantity = input.Quantity
	order.Filled = 0
	order.CreatedAt = types.NowNano()
	order.UpdatedAt = order.CreatedAt
	order.TriggerPrice = input.TriggerPrice
	order.CloseOnTrigger = input.CloseOnTrigger
	order.ReduceOnly = input.ReduceOnly
	order.StopOrderType = input.StopOrderType
	order.IsConditional = input.IsConditional
	order.OrderLinkId = 0

	if input.TriggerPrice > 0 {
		order.Status = constants.ORDER_STATUS_UNTRIGGERED
		s.triggerMon.Add(order)
		s.storeOrder(order)
		s.publishOrderEvent(order)
		result := &types.OrderResult{
			Filled:    0,
			Remaining: order.Quantity,
			Status:    order.Status,
		}
		setOrderResultOrders(result, order)
		return result, nil
	}

	return s.executeOrder(order)
}

func (s *Service) storeOrder(order *types.Order) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.orders[order.UserID]; !ok {
		s.orders[order.UserID] = make(map[types.OrderID]*types.Order)
	}
	s.orders[order.UserID][order.ID] = order
	s.ordersByID[order.ID] = order
}

func (s *Service) removeOrderFromMemory(order *types.Order) {
	if order == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.removeOrderFromMemoryNoLock(order)
}

func (s *Service) removeOrderFromMemoryNoLock(order *types.Order) {
	if order == nil {
		return
	}
	if userOrders, ok := s.orders[order.UserID]; ok {
		delete(userOrders, order.ID)
	}
	delete(s.ordersByID, order.ID)
}

func (s *Service) placeOCOOrder(ctx context.Context, input *types.OrderInput) (*types.OrderResult, error) {
	if err := s.validateOCO(input); err != nil {
		return nil, err
	}

	groupID := int64(poolGetOrderID())

	tpOrder := s.createOCOChildOrder(input, groupID, constants.STOP_ORDER_TYPE_TAKE_PROFIT, input.OCO.TakeProfit)
	slOrder := s.createOCOChildOrder(input, groupID, constants.STOP_ORDER_TYPE_STOP_LOSS, input.OCO.StopLoss)

	s.storeOrder(tpOrder)
	s.storeOrder(slOrder)

	s.triggerMon.Add(tpOrder)
	s.triggerMon.Add(slOrder)

	tpResult := &types.OrderResult{
		Filled:    0,
		Remaining: types.Quantity(input.Quantity),
		Status:    tpOrder.Status,
	}
	setOrderResultOrders(tpResult, tpOrder, slOrder)

	if input.OCO.Quantity > 0 {
		tpOrder.Quantity = input.OCO.Quantity
		slOrder.Quantity = input.OCO.Quantity
	} else {
		pos := s.portfolio.GetPosition(input.UserID, input.Symbol)
		if pos != nil && pos.Size > 0 {
			tpResult.Remaining = types.Quantity(pos.Size)
		}
	}

	s.mu.Lock()
	s.orderLinkIds[tpOrder.ID] = groupID
	s.orderLinkIds[slOrder.ID] = groupID
	s.linkedOrders[groupID] = []types.OrderID{tpOrder.ID, slOrder.ID}
	s.mu.Unlock()

	s.publishOrderEvent(tpOrder)
	s.publishOrderEvent(slOrder)

	return tpResult, nil
}

func (s *Service) createOCOChildOrder(input *types.OrderInput, groupID int64, stopOrderType int8, child types.OCOChildOrder) *types.Order {
	order := pool.GetOrder()
	order.ID = types.OrderID(poolGetOrderID())
	order.UserID = input.UserID
	order.Symbol = input.Symbol
	order.Category = input.Category
	order.Side = input.Side
	order.Type = input.Type
	order.TIF = input.TIF
	order.Status = constants.ORDER_STATUS_UNTRIGGERED
	order.Price = child.Price
	order.Quantity = 0
	order.Filled = 0
	order.CreatedAt = types.NowNano()
	order.UpdatedAt = order.CreatedAt
	order.TriggerPrice = child.TriggerPrice
	order.ReduceOnly = child.ReduceOnly
	order.CloseOnTrigger = true
	order.StopOrderType = stopOrderType
	order.IsConditional = true
	order.OrderLinkId = groupID
	return order
}

func (s *Service) executeOrder(order *types.Order) (*types.OrderResult, error) {
	if order.Type == constants.ORDER_TYPE_LIMIT {
		err := s.clearing.Reserve(order.UserID, order.Symbol, order.Category, order.Side, order.Quantity, order.Price)
		if err != nil {
			pool.PutOrder(order)
			return nil, err
		}
	}

	s.storeOrder(order)

	result := &types.OrderResult{}
	setOrderResultOrders(result, order)
	s.matchOrderInto(order, result)

	order.Filled = order.Quantity - order.Remaining()
	order.UpdatedAt = types.NowNano()

	switch {
	case order.Remaining() == 0 && order.Filled > 0:
		order.Status = constants.ORDER_STATUS_FILLED
	case order.Remaining() == 0 && order.Filled == 0:
		order.Status = constants.ORDER_STATUS_CANCELED
	case order.TIF == constants.TIF_GTC || order.TIF == constants.TIF_POST_ONLY:
		ob := s.getOrderBook(order.Category, order.Symbol)
		ob.Add(order)
		order.Status = constants.ORDER_STATUS_NEW
	case order.TIF == constants.TIF_IOC || order.TIF == constants.TIF_FOK:
		order.Status = constants.ORDER_STATUS_CANCELED
		if order.Filled > 0 {
			order.Status = constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED
		}
	}
	switch order.Status {
	case constants.ORDER_STATUS_NEW:
		s.publishOrderEvent(order)
		if order.ReduceOnly && !order.IsConditional && !order.CloseOnTrigger {
			s.updateReduceOnlyCommitment(order, int64(order.Remaining()))
		}
	case constants.ORDER_STATUS_FILLED:
		if order.ReduceOnly && !order.IsConditional && !order.CloseOnTrigger {
			s.updateReduceOnlyCommitment(order, 0)
		}
		s.publishOrderEvent(order)
		s.removeOrderFromMemory(order)
	case constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED, constants.ORDER_STATUS_CANCELED:
		if order.ReduceOnly && !order.IsConditional && !order.CloseOnTrigger {
			s.updateReduceOnlyCommitment(order, 0)
		}
		if order.Remaining() > 0 {
			s.clearing.Release(order.UserID, order.Symbol, order.Category, order.Side, order.Remaining(), order.Price)
		}
		s.publishOrderEvent(order)
		s.removeOrderFromMemory(order)
	}
	result.Filled = order.Filled
	result.Remaining = order.Remaining()
	result.Status = order.Status
	return result, nil
}

func (s *Service) cancelOrder(order *types.Order) {
	if order.Status == constants.ORDER_STATUS_UNTRIGGERED {
		s.triggerMon.Remove(order.ID)
	}
	if order.ReduceOnly && !order.IsConditional && !order.CloseOnTrigger {
		s.updateReduceOnlyCommitment(order, 0)
	}

	ob := s.getOrderBookIfExists(order.Category, order.Symbol)
	if ob != nil {
		ob.Remove(order.ID)
	}

	s.removeOrderFromMemory(order)

	if order.Filled > 0 {
		order.Status = constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED
	} else {
		order.Status = constants.ORDER_STATUS_CANCELED
	}
	order.UpdatedAt = types.NowNano()

	s.publishOrderEvent(order)

	pool.PutOrder(order)
}
