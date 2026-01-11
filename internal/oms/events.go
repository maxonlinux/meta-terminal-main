package oms

import (
	"context"

	"github.com/anomalyco/meta-terminal-go/internal/events"
	"github.com/anomalyco/meta-terminal-go/internal/messaging"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func (s *Service) publishOrderEvent(order *types.Order) {
	if s.nats == nil {
		if !s.hasSink() {
			return
		}
		s.publishOrderToSink(order)
		return
	}
	event := &types.OrderEvent{
		OrderID:      order.ID,
		UserID:       order.UserID,
		Symbol:       order.Symbol,
		Category:     order.Category,
		Side:         order.Side,
		Type:         order.Type,
		TIF:          order.TIF,
		Status:       order.Status,
		Price:        order.Price,
		Quantity:     order.Quantity,
		Filled:       order.Filled,
		TriggerPrice: order.TriggerPrice,
		ReduceOnly:   order.ReduceOnly,
		OrderLinkId:  order.OrderLinkId,
		CreatedAt:    order.CreatedAt,
		UpdatedAt:    order.UpdatedAt,
	}
	_ = s.nats.PublishGob(context.Background(), messaging.OrderEventTopic(order.Symbol), event)
	if s.hasSink() {
		s.sink.OnOrderEvent(event)
	}
}

func (s *Service) publishOrderToSink(order *types.Order) {
	event := &types.OrderEvent{
		OrderID:      order.ID,
		UserID:       order.UserID,
		Symbol:       order.Symbol,
		Category:     order.Category,
		Side:         order.Side,
		Type:         order.Type,
		TIF:          order.TIF,
		Status:       order.Status,
		Price:        order.Price,
		Quantity:     order.Quantity,
		Filled:       order.Filled,
		TriggerPrice: order.TriggerPrice,
		ReduceOnly:   order.ReduceOnly,
		OrderLinkId:  order.OrderLinkId,
		CreatedAt:    order.CreatedAt,
		UpdatedAt:    order.UpdatedAt,
	}
	s.sink.OnOrderEvent(event)
}

func (s *Service) hasSink() bool {
	_, ok := s.sink.(events.NopSink)
	return !ok
}
