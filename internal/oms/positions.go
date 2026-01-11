package oms

import (
	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/pool"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func (s *Service) OnPositionUpdate(userID types.UserID, symbol string, newSize int64, newSide int8) {
	s.posMu.Lock()
	defer s.posMu.Unlock()
	var toDeactivate []*types.Order
	seenGroups := make(map[int64]struct{})
	var reduceOnlyOrders []*types.Order
	var closeOnTriggerOrders []*types.Order
	allowed := absInt64(newSize)

	s.mu.Lock()
	if userOrders, ok := s.orders[userID]; ok {
		for _, order := range userOrders {
			if order.Symbol != symbol {
				continue
			}
			if order.Status != constants.ORDER_STATUS_UNTRIGGERED {
				if order.ReduceOnly && order.Status == constants.ORDER_STATUS_NEW {
					reduceOnlyOrders = append(reduceOnlyOrders, order)
				}
				continue
			}
			if order.OrderLinkId <= 0 && !order.CloseOnTrigger {
				continue
			}
			if order.CloseOnTrigger && order.Quantity > 0 {
				closeOnTriggerOrders = append(closeOnTriggerOrders, order)
			}
			toDeactivate = append(toDeactivate, order)
			if order.OrderLinkId > 0 {
				seenGroups[order.OrderLinkId] = struct{}{}
			}
		}
	}
	s.mu.Unlock()

	if len(reduceOnlyOrders) > 0 {
		s.adjustReduceOnlyOrders(userID, symbol, reduceOnlyOrders, allowed)
	}
	if len(closeOnTriggerOrders) > 0 {
		s.adjustCloseOnTriggerOrders(closeOnTriggerOrders, allowed)
	}

	if newSize != 0 {
		return
	}

	for _, order := range toDeactivate {
		s.triggerMon.Remove(order.ID)
		order.Status = constants.ORDER_STATUS_DEACTIVATED
		order.UpdatedAt = types.NowNano()
		s.publishOrderEvent(order)
		if order.OrderLinkId > 0 {
			s.mu.Lock()
			delete(s.orderLinkIds, order.ID)
			if _, ok := seenGroups[order.OrderLinkId]; ok {
				delete(s.linkedOrders, order.OrderLinkId)
				delete(seenGroups, order.OrderLinkId)
			}
			s.mu.Unlock()
		}
		s.removeOrderFromMemory(order)
		pool.PutOrder(order)
	}
}

func (s *Service) updateReduceOnlyCommitment(order *types.Order, newValue int64) {
	if order == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	old := s.reduceOnlyByOrder[order.ID]
	if newValue == old {
		return
	}
	if newValue == 0 {
		delete(s.reduceOnlyByOrder, order.ID)
	} else {
		s.reduceOnlyByOrder[order.ID] = newValue
	}
	if _, ok := s.reduceOnlyCommitment[order.UserID]; !ok {
		s.reduceOnlyCommitment[order.UserID] = make(map[string]int64)
	}
	s.reduceOnlyCommitment[order.UserID][order.Symbol] += newValue - old
	if s.reduceOnlyCommitment[order.UserID][order.Symbol] < 0 {
		s.reduceOnlyCommitment[order.UserID][order.Symbol] = 0
	}
}

func (s *Service) adjustReduceOnlyOrders(userID types.UserID, symbol string, orders []*types.Order, allowed int64) {
	if allowed < 0 {
		allowed = 0
	}
	for _, order := range orders {
		if allowed == 0 {
			allowed = 0
		}
		remaining := int64(order.Remaining())
		newRemaining := remaining
		if remaining > allowed {
			newRemaining = allowed
		}
		allowed -= newRemaining
		if newRemaining != remaining {
			ob := s.getOrderBook(order.Category, order.Symbol)
			_ = ob.Adjust(order.ID, types.Quantity(newRemaining))
			order.UpdatedAt = types.NowNano()
		}
		s.updateReduceOnlyCommitment(order, newRemaining)
	}
}

func (s *Service) adjustCloseOnTriggerOrders(orders []*types.Order, allowed int64) {
	if allowed < 0 {
		allowed = 0
	}
	for _, order := range orders {
		if order.Quantity == 0 {
			continue
		}
		remaining := int64(order.Quantity)
		newRemaining := remaining
		if remaining > allowed {
			newRemaining = allowed
		}
		allowed -= newRemaining
		if newRemaining != remaining {
			order.Quantity = types.Quantity(newRemaining)
			order.UpdatedAt = types.NowNano()
		}
	}
}
