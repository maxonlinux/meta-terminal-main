package oms

import (
	"context"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/messaging"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func (s *Service) matchOrder(order *types.Order) []*types.Trade {
	ob := s.getOrderBook(order.Category, order.Symbol)

	var limitPrice types.Price
	if order.Type == constants.ORDER_TYPE_LIMIT {
		limitPrice = order.Price
	}

	matches, _ := ob.Match(order, limitPrice)

	trades := make([]*types.Trade, 0, len(matches))
	for _, match := range matches {
		trades = append(trades, match.Trade)
		s.clearing.ExecuteTrade(match.Trade, order, match.Maker)
		s.publishTrade(match.Trade)
	}

	return trades
}

func (s *Service) publishTrade(trade *types.Trade) {
	if s.nats == nil {
		return
	}
	event := &types.TradeEvent{
		TradeID:      trade.ID,
		Symbol:       trade.Symbol,
		Category:     trade.Category,
		TakerID:      trade.TakerID,
		MakerID:      trade.MakerID,
		TakerOrderID: trade.TakerOrderID,
		MakerOrderID: trade.MakerOrderID,
		Price:        trade.Price,
		Quantity:     trade.Quantity,
		ExecutedAt:   trade.ExecutedAt,
	}
	s.nats.PublishGob(context.Background(), messaging.SubjectClearingTrade, event)
}
