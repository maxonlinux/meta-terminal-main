package oms

import (
	"context"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/messaging"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func (s *Service) matchOrderInto(order *types.Order, result *types.OrderResult) []types.Trade {
	ob := s.getOrderBook(order.Category, order.Symbol)

	var limitPrice types.Price
	if order.Type == constants.ORDER_TYPE_LIMIT {
		limitPrice = order.Price
	}

	bufAny := s.matchBufPool.Get()
	matchesBuf, _ := bufAny.([]types.Match)
	matches, _ := ob.MatchInto(order, limitPrice, matchesBuf)

	var trades []types.Trade
	if result != nil && len(matches) <= len(result.TradesBuf) {
		trades = result.TradesBuf[:0]
	} else {
		trades = make([]types.Trade, 0, len(matches))
	}

	for i := range matches {
		trades = append(trades, matches[i].Trade)
		tradePtr := &trades[len(trades)-1]
		s.clearing.ExecuteTrade(tradePtr, order, matches[i].Maker)
		s.publishTrade(tradePtr)
		matches[i] = types.Match{}
	}
	s.matchBufPool.Put(matches[:0])

	if result != nil {
		result.Trades = trades
	}
	return trades
}

func (s *Service) publishTrade(trade *types.Trade) {
	if s.nats == nil && !s.hasSink() {
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
	if s.nats != nil {
		s.nats.PublishGob(context.Background(), messaging.SUBJECT_CLEARING_TRADE, event)
	}
	if s.hasSink() {
		s.sink.OnTradeEvent(event)
	}
}
