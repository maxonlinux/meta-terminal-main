package balance

import (
	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func Lock(s *state.State, userID types.UserID, asset string, amount int64) error {
	us := s.GetUserState(userID)
	balance, ok := us.Balances[asset]
	if !ok {
		balance = &types.UserBalance{
			UserID:    userID,
			Asset:     asset,
			Available: 0,
			Locked:    0,
			Margin:    0,
			Version:   0,
		}
		us.Balances[asset] = balance
	}

	if balance.Available < amount {
		return ErrInsufficientBalance
	}

	balance.Available -= amount
	balance.Locked += amount
	balance.Version++

	return nil
}

func Unlock(s *state.State, userID types.UserID, asset string, amount int64) error {
	us := s.GetUserState(userID)
	balance, ok := us.Balances[asset]
	if !ok {
		return ErrBalanceNotFound
	}

	if balance.Locked < amount {
		return ErrInvalidUnlockAmount
	}

	balance.Locked -= amount
	balance.Available += amount
	balance.Version++

	return nil
}

func TransferToMargin(s *state.State, userID types.UserID, asset string, amount int64) error {
	us := s.GetUserState(userID)
	balance, ok := us.Balances[asset]
	if !ok {
		return ErrBalanceNotFound
	}

	if balance.Available < amount {
		return ErrInsufficientBalance
	}

	balance.Available -= amount
	balance.Margin += amount
	balance.Version++

	return nil
}

func TransferFromMargin(s *state.State, userID types.UserID, asset string, amount int64) error {
	us := s.GetUserState(userID)
	balance, ok := us.Balances[asset]
	if !ok {
		return ErrBalanceNotFound
	}

	if balance.Margin < amount {
		return ErrInvalidMarginAmount
	}

	balance.Margin -= amount
	balance.Available += amount
	balance.Version++

	return nil
}

func GetAvailable(s *state.State, userID types.UserID, asset string) int64 {
	us, ok := s.Users[userID]
	if !ok {
		return 0
	}
	balance, ok := us.Balances[asset]
	if !ok {
		return 0
	}
	return balance.Available
}

func GetLocked(s *state.State, userID types.UserID, asset string) int64 {
	us, ok := s.Users[userID]
	if !ok {
		return 0
	}
	balance, ok := us.Balances[asset]
	if !ok {
		return 0
	}
	return balance.Locked
}

func GetMargin(s *state.State, userID types.UserID, asset string) int64 {
	us, ok := s.Users[userID]
	if !ok {
		return 0
	}
	balance, ok := us.Balances[asset]
	if !ok {
		return 0
	}
	return balance.Margin
}
