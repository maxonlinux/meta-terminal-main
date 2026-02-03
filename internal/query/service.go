package query

import (
	"github.com/maxonlinux/meta-terminal-go/internal/oms"
	orderbook "github.com/maxonlinux/meta-terminal-go/internal/orderbook"
	"github.com/maxonlinux/meta-terminal-go/internal/portfolio"
	"github.com/maxonlinux/meta-terminal-go/internal/registry"
	"github.com/maxonlinux/meta-terminal-go/internal/trades"
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

type Service struct {
	registry  *registry.Registry
	portfolio *portfolio.Service
	store     *oms.Service
	trades    *trades.TradeFeed
	getBook   func(category int8, symbol string) *orderbook.OrderBook
}

func New(reg *registry.Registry, portfolio *portfolio.Service, store *oms.Service, trades *trades.TradeFeed, getBook func(category int8, symbol string) *orderbook.OrderBook) *Service {
	return &Service{
		registry:  reg,
		portfolio: portfolio,
		store:     store,
		trades:    trades,
		getBook:   getBook,
	}
}

func (s *Service) GetOrders(userID types.UserID, symbol string, category *int8, filter string, limit int, offset int) []*types.Order {
	orders := s.store.GetUserOrders(userID)
	if symbol == "" && category == nil && filter == "" && limit <= 0 && offset <= 0 {
		return orders
	}
	result := make([]*types.Order, 0, len(orders))
	for _, o := range orders {
		if o.Origin == constants.ORDER_ORIGIN_SYSTEM {
			continue
		}
		if symbol != "" && o.Symbol != symbol {
			continue
		}
		if category != nil && o.Category != *category {
			continue
		}
		if filter != "" && filter != "all" {
			if filter == "open" {
				if !isOrderOpen(o.Status) {
					continue
				}
			} else if filter == "closed" {
				if isOrderOpen(o.Status) {
					continue
				}
			}
		}
		result = append(result, o)
	}
	if offset > 0 {
		if offset >= len(result) {
			return nil
		}
		result = result[offset:]
	}
	if limit > 0 && limit < len(result) {
		result = result[:limit]
	}
	return result
}

func (s *Service) GetOrder(userID types.UserID, id types.OrderID) (*types.Order, bool) {
	order, ok := s.store.GetUserOrder(userID, id)
	if !ok || order.Origin == constants.ORDER_ORIGIN_SYSTEM {
		return nil, false
	}
	return order, true
}

func isOrderOpen(status int8) bool {
	return status == constants.ORDER_STATUS_NEW ||
		status == constants.ORDER_STATUS_PARTIALLY_FILLED ||
		status == constants.ORDER_STATUS_UNTRIGGERED ||
		status == constants.ORDER_STATUS_TRIGGERED
}

func (s *Service) GetBalances(userID types.UserID) []*types.Balance {
	return s.portfolio.GetBalances(userID)
}

func (s *Service) GetBalance(userID types.UserID, asset string) *types.Balance {
	return s.portfolio.GetBalance(userID, asset)
}

func (s *Service) GetPositions(userID types.UserID) []*types.Position {
	return s.portfolio.GetPositions(userID)
}

func (s *Service) GetInstruments(symbol string) []*types.Instrument {
	if symbol != "" {
		inst := s.registry.GetInstrument(symbol)
		if inst != nil {
			return []*types.Instrument{inst}
		}
		return []*types.Instrument{}
	}
	return s.registry.GetInstruments()
}

type OrderBookSnapshot struct {
	Symbol   string
	Category int8
	Bids     []orderbook.LevelSnapshot
	Asks     []orderbook.LevelSnapshot
}

func (s *Service) GetOrderBook(category int8, symbol string, depth int) *OrderBookSnapshot {
	book := s.getBook(category, symbol)
	if book == nil {
		return nil
	}
	if depth <= 0 {
		depth = 50
	}
	snap := book.Snapshot(depth)
	return &OrderBookSnapshot{Symbol: symbol, Category: category, Bids: snap.Bids, Asks: snap.Asks}
}

func (s *Service) GetPublicTrades(category int8, symbol string) []types.Trade {
	return s.trades.Recent(category, symbol)
}
