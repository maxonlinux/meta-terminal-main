package oms

import (
	"context"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/pool"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func (s *Service) OnPriceTick(symbol string, price types.Price) {
	s.mu.Lock()
	s.lastPrices[symbol] = price
	s.mu.Unlock()

	bufAny := s.triggerBufPool.Get()
	buf, _ := bufAny.(*[]*types.Order)
	if buf == nil {
		empty := make([]*types.Order, 0, 32)
		buf = &empty
	}
	triggered := s.triggerMon.CheckInto(price, *buf)
	for _, order := range triggered {
		if order.Status == constants.ORDER_STATUS_DEACTIVATED {
			continue
		}
		if order.OrderLinkId > 0 {
			s.deactivateLinkedOrders(order)
		}

		s.handleTrigger(order)
	}
	for i := range triggered {
		triggered[i] = nil
	}
	*buf = triggered[:0]
	s.triggerBufPool.Put(buf)
}

func (s *Service) handleTrigger(order *types.Order) {
	s.removeOrderFromMemory(order)

	childInput := s.createChildOrderInput(order)
	if childInput != nil {
		_, _ = s.PlaceOrder(context.Background(), childInput)
	}
}

func (s *Service) createChildOrderInput(triggered *types.Order) *types.OrderInput {
	if triggered.CloseOnTrigger {
		pos := s.portfolio.GetPosition(triggered.UserID, triggered.Symbol)
		if pos.Size == 0 {
			return nil
		}

		qty := triggered.Quantity
		if qty == 0 {
			qty = types.Quantity(absInt64(pos.Size))
		} else if int64(qty) > absInt64(pos.Size) {
			qty = types.Quantity(absInt64(pos.Size))
		}

		var tif int8
		var price types.Price
		if triggered.Type == constants.ORDER_TYPE_MARKET {
			tif = constants.TIF_IOC
		} else {
			tif = constants.TIF_GTC
			price = triggered.Price
		}

		side := int8(constants.ORDER_SIDE_SELL)
		if pos.Side == constants.SIDE_SHORT {
			side = constants.ORDER_SIDE_BUY
		}
		return &types.OrderInput{
			UserID:     triggered.UserID,
			Symbol:     triggered.Symbol,
			Category:   triggered.Category,
			Side:       side,
			Type:       triggered.Type,
			TIF:        tif,
			Quantity:   qty,
			Price:      price,
			ReduceOnly: true,
		}
	}

	return &types.OrderInput{
		UserID:        triggered.UserID,
		Symbol:        triggered.Symbol,
		Category:      triggered.Category,
		Side:          triggered.Side,
		Type:          triggered.Type,
		TIF:           triggered.TIF,
		Quantity:      triggered.Quantity,
		Price:         triggered.Price,
		TriggerPrice:  0,
		ReduceOnly:    triggered.ReduceOnly,
		StopOrderType: triggered.StopOrderType,
	}
}

func (s *Service) deactivateLinkedOrders(triggered *types.Order) {
	s.mu.RLock()
	groupID, ok := s.orderLinkIds[triggered.ID]
	if !ok {
		s.mu.RUnlock()
		return
	}
	linkedIDs := s.linkedOrders[groupID]
	s.mu.RUnlock()

	for _, linkedID := range linkedIDs {
		if linkedID == triggered.ID {
			continue
		}

		s.mu.RLock()
		linkedOrder := s.ordersByID[linkedID]
		s.mu.RUnlock()

		if linkedOrder == nil || linkedOrder.Status != constants.ORDER_STATUS_UNTRIGGERED {
			continue
		}

		s.triggerMon.Remove(linkedOrder.ID)
		linkedOrder.Status = constants.ORDER_STATUS_DEACTIVATED
		linkedOrder.UpdatedAt = types.NowNano()
		s.publishOrderEvent(linkedOrder)
		s.removeOrderFromMemory(linkedOrder)
		pool.PutOrder(linkedOrder)
	}

	s.mu.Lock()
	delete(s.orderLinkIds, triggered.ID)
	delete(s.linkedOrders, groupID)
	s.mu.Unlock()
}
