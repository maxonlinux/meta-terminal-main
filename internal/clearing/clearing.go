package clearing

import (
	"github.com/maxonlinux/meta-terminal-go/internal/constants"
	sym "github.com/maxonlinux/meta-terminal-go/pkg/symbol"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

type Portfolio interface {
	Reserve(uid types.UserID, asset string, amount types.Quantity) error
	Release(uid types.UserID, asset string, amount types.Quantity)
	ExecuteTrade(t *types.Trade, taker, maker *types.Order)
	GetPosition(uid types.UserID, symbol string) *types.Position
}

type Service struct{ p Portfolio }

func New(p Portfolio) *Service { return &Service{p: p} }

func (s *Service) Reserve(uid types.UserID, symbol string, cat, side int8, qty types.Quantity, price types.Price) error {
	amt, asset := s.calcReserve(uid, symbol, cat, side, qty, price)
	return s.p.Reserve(uid, asset, amt)
}

func (s *Service) Release(uid types.UserID, symbol string, cat, side int8, qty types.Quantity, price types.Price) {
	amt, asset := s.calcReserve(uid, symbol, cat, side, qty, price)
	s.p.Release(uid, asset, amt)
}

func (s *Service) calcReserve(uid types.UserID, symbol string, cat, side int8, qty types.Quantity, price types.Price) (types.Quantity, string) {
	if cat == constants.CATEGORY_SPOT {
		if side == constants.ORDER_SIDE_BUY {
			return qty * types.Quantity(price), sym.GetQuoteAsset(symbol)
		}
		return qty, sym.GetBaseAsset(symbol)
	}
	p := s.p.GetPosition(uid, symbol)
	lev := p.Leverage
	if lev <= 0 {
		lev = constants.DEFAULT_LEVERAGE
	}
	return (qty * types.Quantity(price)) / types.Quantity(lev), sym.GetQuoteAsset(symbol)
}

func (s *Service) ExecuteTrade(t *types.Trade, taker, maker *types.Order) {
	if t.Category == constants.CATEGORY_SPOT {
		s.p.ExecuteTrade(t, taker, maker)
	} else {
		s.p.ExecuteTrade(t, taker, maker)
	}
}
