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

func (s *Service) GetOrders(userID types.UserID, symbol string, category int8) []*types.Order {
	var result []*types.Order
	s.store.Iterate(func(o *types.Order) bool {
		if o.UserID != userID {
			return true
		}
		if symbol != "" && o.Symbol != symbol {
			return true
		}
		if category != 0 && o.Category != category {
			return true
		}
		result = append(result, o)
		return true
	})
	return result
}

func (s *Service) GetOrder(id types.OrderID) (*types.Order, bool) {
	return s.store.Get(id)
}

func (s *Service) GetBalances(userID types.UserID) []*types.Balance {
	return s.portfolio.GetBalances(userID)
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
	Symbol string
	Bids   []orderbook.LevelSnapshot
	Asks   []orderbook.LevelSnapshot
}

func (s *Service) GetOrderBook(symbol string) *OrderBookSnapshot {
	book := s.getBook(constants.CATEGORY_SPOT, symbol)
	if book == nil {
		return nil
	}
	snap := book.Snapshot(50)
	return &OrderBookSnapshot{Symbol: symbol, Bids: snap.Bids, Asks: snap.Asks}
}

func (s *Service) GetPublicTrades(symbol string) []types.Trade {
	return s.trades.Recent(constants.CATEGORY_SPOT, symbol)
}
