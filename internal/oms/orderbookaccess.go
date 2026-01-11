package oms

import (
	"github.com/anomalyco/meta-terminal-go/internal/orderbook"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func (s *Service) getOrderBook(category int8, symbol string) *orderbook.OrderBook {
	if ob, ok := s.orderbooks[category][symbol]; ok {
		return ob
	}
	ob := orderbook.New()
	s.orderbooks[category][symbol] = ob
	return ob
}

func (s *Service) getOrderBookIfExists(category int8, symbol string) *orderbook.OrderBook {
	return s.orderbooks[category][symbol]
}

func (s *Service) GetOrderBook(category int8, symbol string) (bidPrice types.Price, bidQty types.Quantity, askPrice types.Price, askQty types.Quantity) {
	ob := s.getOrderBookIfExists(category, symbol)
	if ob == nil {
		return 0, 0, 0, 0
	}
	bidPrice, bidQty, _ = ob.BestBid()
	askPrice, askQty, _ = ob.BestAsk()
	return
}

func (s *Service) GetLastPrice(symbol string) types.Price {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastPrices[symbol]
}

func (s *Service) GetOrderBookDepth(category int8, symbol string, limit int) ([]types.Price, []types.Quantity, []types.Price, []types.Quantity) {
	ob := s.getOrderBookIfExists(category, symbol)
	if ob == nil {
		return nil, nil, nil, nil
	}
	return ob.Depth(limit)
}
