package portfolio

import (
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

func (s *Service) GetBalance(userID types.UserID, asset string) *types.Balance {
	return s.balanceFor(userID, asset)
}

func (s *Service) Reserve(userID types.UserID, asset string, amount types.Quantity) error {
	balance := s.balanceFor(userID, asset)
	if math.Lt(balance.Available, amount) {
		return constants.ErrInsufficientBalance
	}
	balance.Available = math.Sub(balance.Available, amount)
	balance.Locked = math.Add(balance.Locked, amount)
	return nil
}

func (s *Service) Release(userID types.UserID, asset string, amount types.Quantity) {
	s.adjustLocked(userID, asset, math.Neg(amount))
	s.adjustAvailable(userID, asset, amount)
}

func (s *Service) adjustAvailable(userID types.UserID, asset string, delta types.Quantity) {
	balance := s.balanceFor(userID, asset)
	balance.Available = math.Add(balance.Available, delta)
	if s.store != nil {
		s.store.SaveBalance(balance)
	}
}

func (s *Service) adjustLocked(userID types.UserID, asset string, delta types.Quantity) {
	balance := s.balanceFor(userID, asset)
	balance.Locked = math.Add(balance.Locked, delta)
	if math.Sign(balance.Locked) < 0 {
		balance.Locked = math.Zero
	}
	if s.store != nil {
		s.store.SaveBalance(balance)
	}
}

func (s *Service) adjustMargin(userID types.UserID, asset string, delta types.Quantity) {
	balance := s.balanceFor(userID, asset)
	balance.Margin = math.Add(balance.Margin, delta)
	if s.store != nil {
		s.store.SaveBalance(balance)
	}
}

func (s *Service) balanceFor(userID types.UserID, asset string) *types.Balance {
	balances := s.Balances[userID]
	if balances == nil {
		balances = make(map[string]*types.Balance)
		s.Balances[userID] = balances
	}

	balance, ok := balances[asset]
	if !ok {
		balance = &types.Balance{UserID: userID, Asset: asset, Available: math.Zero, Locked: math.Zero, Margin: math.Zero}
		balances[asset] = balance
	}
	return balance
}
