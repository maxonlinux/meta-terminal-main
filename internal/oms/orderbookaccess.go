package oms

import (
	"github.com/anomalyco/meta-terminal-go/internal/orderbook"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

// getOrderBook retrieves an existing OrderBook for the given category and symbol,
// or creates a new one if it doesn't exist.
func (s *Service) getOrderBook(category int8, symbol string) *orderbook.OrderBook {
	s.mu.RLock()
	categoryMap := s.orderbooks[category]
	if categoryMap == nil {
		s.mu.RUnlock()
		s.mu.Lock()
		if s.orderbooks[category] == nil {
			s.orderbooks[category] = make(map[string]*orderbook.OrderBook)
		}
		categoryMap = s.orderbooks[category]
		s.mu.Unlock()
	}
	ob := categoryMap[symbol]
	s.mu.RUnlock()

	if ob != nil {
		return ob
	}

	s.mu.Lock()
	categoryMap = s.orderbooks[category]
	ob = categoryMap[symbol]
	if ob == nil {
		ob = orderbook.New()
		categoryMap[symbol] = ob
	}
	s.mu.Unlock()
	return ob
}

// getOrderBookIfExists returns the OrderBook for the given category and symbol,
// or nil if it doesn't exist.
func (s *Service) getOrderBookIfExists(category int8, symbol string) *orderbook.OrderBook {
	s.mu.RLock()
	categoryMap := s.orderbooks[category]
	if categoryMap == nil {
		s.mu.RUnlock()
		return nil
	}
	ob := categoryMap[symbol]
	s.mu.RUnlock()
	return ob
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
