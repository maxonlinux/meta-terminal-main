package portfolio

import (
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

func (s *Service) GetBalance(userID types.UserID, asset string) *types.Balance {
	s.mu.Lock()
	defer s.mu.Unlock()
	balance := s.balanceForLocked(userID, asset)
	if balance == nil {
		return nil
	}
	copy := *balance
	return &copy
}

func (s *Service) Reserve(userID types.UserID, asset string, amount types.Quantity) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reserveLocked(userID, asset, amount)
}

func (s *Service) Release(userID types.UserID, asset string, amount types.Quantity) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.releaseLocked(userID, asset, amount)
}

// reserveLocked expects the portfolio mutex to be held.
func (s *Service) reserveLocked(userID types.UserID, asset string, amount types.Quantity) error {
	balance := s.balanceForLocked(userID, asset)
	if math.Lt(balance.Available, amount) {
		return constants.ErrInsufficientBalance
	}
	balance.Available = math.Sub(balance.Available, amount)
	balance.Locked = math.Add(balance.Locked, amount)
	if s.onBalance != nil {
		s.onBalance(userID, asset, balance)
	}
	return nil
}

// releaseLocked expects the portfolio mutex to be held.
func (s *Service) releaseLocked(userID types.UserID, asset string, amount types.Quantity) {
	s.adjustLocked(userID, asset, math.Neg(amount))
	s.adjustAvailable(userID, asset, amount)
}

// adjustAvailable expects the portfolio mutex to be held.
func (s *Service) adjustAvailable(userID types.UserID, asset string, delta types.Quantity) {
	balance := s.balanceForLocked(userID, asset)
	balance.Available = math.Add(balance.Available, delta)
	if s.onBalance != nil {
		s.onBalance(userID, asset, balance)
	}
}

// adjustLocked expects the portfolio mutex to be held.
func (s *Service) adjustLocked(userID types.UserID, asset string, delta types.Quantity) {
	balance := s.balanceForLocked(userID, asset)
	balance.Locked = math.Add(balance.Locked, delta)
	if math.Sign(balance.Locked) < 0 {
		balance.Locked = math.Zero
	}
	if s.onBalance != nil {
		s.onBalance(userID, asset, balance)
	}
}

// adjustMargin expects the portfolio mutex to be held.
func (s *Service) adjustMargin(userID types.UserID, asset string, delta types.Quantity) {
	balance := s.balanceForLocked(userID, asset)
	balance.Margin = math.Add(balance.Margin, delta)
	if s.onBalance != nil {
		s.onBalance(userID, asset, balance)
	}
}

// balanceForLocked expects the portfolio mutex to be held.
func (s *Service) balanceForLocked(userID types.UserID, asset string) *types.Balance {
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
