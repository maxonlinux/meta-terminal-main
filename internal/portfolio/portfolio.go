package portfolio

import (
	"errors"

	"github.com/maxonlinux/meta-terminal-go/internal/constants"
	"github.com/maxonlinux/meta-terminal-go/internal/types"
	sym "github.com/maxonlinux/meta-terminal-go/pkg/symbol"
)

var (
	ErrInsufficientBalance = errors.New("insufficient balance")
	ErrInvalidLeverage     = errors.New("leverage must be 2-100")
)

type Portfolio interface {
	ExecuteTrade(t *types.Trade, taker, maker *types.Order)
}

type Service struct {
	portfolio Portfolio
	balances  map[types.UserID]map[string]*types.UserBalance
	positions map[types.UserID]map[string]*types.Position
}

func New(p Portfolio) *Service {
	return &Service{
		portfolio: p,
		balances:  make(map[types.UserID]map[string]*types.UserBalance),
		positions: make(map[types.UserID]map[string]*types.Position),
	}
}

func (s *Service) GetBalance(uid types.UserID, asset string) *types.UserBalance {
	return s.getOrCreateBalance(uid, asset)
}

func (s *Service) Reserve(uid types.UserID, asset string, amount types.Quantity) error {
	b := s.getOrCreateBalance(uid, asset)
	if b.Available < amount {
		return ErrInsufficientBalance
	}
	b.Available -= amount
	b.Locked += amount
	return nil
}

func (s *Service) Release(uid types.UserID, asset string, amount types.Quantity) {
	b := s.getOrCreateBalance(uid, asset)
	if b.Locked >= amount {
		b.Locked -= amount
		b.Available += amount
	} else {
		b.Available += b.Locked
		b.Locked = 0
	}
}

func (s *Service) ExecuteTrade(t *types.Trade, taker, maker *types.Order) {
	if t.Category == constants.CATEGORY_SPOT {
		s.spotTrade(t, taker, maker)
	} else {
		s.linearTrade(t, taker, maker)
	}
	if s.portfolio != nil {
		s.portfolio.ExecuteTrade(t, taker, maker)
	}
}

func (s *Service) GetPosition(uid types.UserID, symbol string) *types.Position {
	return s.getOrCreatePosition(uid, symbol)
}

func (s *Service) SetLeverage(uid types.UserID, symbol string, lev int) error {
	if lev < 2 || lev > 100 {
		return ErrInvalidLeverage
	}
	s.getOrCreatePosition(uid, symbol).Leverage = lev
	return nil
}

func (s *Service) spotTrade(t *types.Trade, taker, maker *types.Order) {
	base := sym.GetBaseAsset(t.Symbol)
	quote := sym.GetQuoteAsset(t.Symbol)
	cost := types.Quantity(t.Price) * t.Quantity

	if taker.Side == constants.ORDER_SIDE_BUY {
		s.getOrCreateBalance(taker.UserID, quote).Available -= cost
		s.getOrCreateBalance(taker.UserID, base).Available += t.Quantity
		s.getOrCreateBalance(maker.UserID, quote).Available += cost
		s.getOrCreateBalance(maker.UserID, base).Available -= t.Quantity
	} else {
		s.getOrCreateBalance(taker.UserID, quote).Available += cost
		s.getOrCreateBalance(taker.UserID, base).Available -= t.Quantity
		s.getOrCreateBalance(maker.UserID, quote).Available -= cost
		s.getOrCreateBalance(maker.UserID, base).Available += t.Quantity
	}
}

func (s *Service) linearTrade(t *types.Trade, taker, maker *types.Order) {
	s.updatePosition(taker.UserID, t.Symbol, t.Quantity, taker.Side == constants.ORDER_SIDE_BUY)
	if maker != nil {
		s.updatePosition(maker.UserID, t.Symbol, t.Quantity, maker.Side == constants.ORDER_SIDE_BUY)
	}
}

func (s *Service) updatePosition(uid types.UserID, symbol string, qty types.Quantity, isBuy bool) {
	p := s.getOrCreatePosition(uid, symbol)

	if p.Size == 0 {
		p.Size = qty
		p.Side = constants.SIDE_LONG
		if !isBuy {
			p.Size = -qty
			p.Side = constants.SIDE_SHORT
		}
		return
	}

	if (p.Side == constants.SIDE_LONG && isBuy) || (p.Side == constants.SIDE_SHORT && !isBuy) {
		p.Size += qty
	} else {
		p.Size -= qty
		if p.Size == 0 {
			p.Side = constants.SIDE_NONE
		} else if p.Size > 0 {
			p.Side = constants.SIDE_LONG
		} else {
			p.Side = constants.SIDE_SHORT
		}
	}
}

func (s *Service) getOrCreateBalance(uid types.UserID, asset string) *types.UserBalance {
	if s.balances[uid] == nil {
		s.balances[uid] = make(map[string]*types.UserBalance)
	}
	if s.balances[uid][asset] == nil {
		s.balances[uid][asset] = &types.UserBalance{UserID: uid, Asset: asset}
	}
	return s.balances[uid][asset]
}

func (s *Service) getOrCreatePosition(uid types.UserID, symbol string) *types.Position {
	if s.positions[uid] == nil {
		s.positions[uid] = make(map[string]*types.Position)
	}
	if s.positions[uid][symbol] == nil {
		s.positions[uid][symbol] = &types.Position{UserID: uid, Symbol: symbol, Leverage: constants.DEFAULT_LEVERAGE}
	}
	return s.positions[uid][symbol]
}
