package balance

import (
	"errors"

	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/types"
)

var (
	ErrInsufficientBalance = errors.New("insufficient balance")
	ErrNegativeAmount      = errors.New("amount cannot be negative")
)

func Add(s *state.EngineState, userID types.UserID, asset string, bucket int8, amount int64) error {
	if amount < 0 {
		return ErrNegativeAmount
	}
	us := s.GetUserState(userID)
	bal, ok := us.Balances[asset]
	if !ok {
		bal = types.NewUserBalance(userID, asset)
		us.Balances[asset] = bal
	}
	bal.Add(bucket, amount)
	bal.Version++
	return nil
}

func Deduct(s *state.EngineState, userID types.UserID, asset string, bucket int8, amount int64) error {
	if amount < 0 {
		return ErrNegativeAmount
	}
	us := s.GetUserState(userID)
	bal, ok := us.Balances[asset]
	if !ok {
		return ErrInsufficientBalance
	}
	if !bal.Deduct(bucket, amount) {
		return ErrInsufficientBalance
	}
	bal.Version++
	return nil
}

func Move(s *state.EngineState, userID types.UserID, asset string, fromBucket, toBucket int8, amount int64) error {
	if amount < 0 {
		return ErrNegativeAmount
	}
	us := s.GetUserState(userID)
	bal, ok := us.Balances[asset]
	if !ok {
		return ErrInsufficientBalance
	}
	if bal.Buckets[fromBucket] < amount {
		return ErrInsufficientBalance
	}
	bal.Buckets[fromBucket] -= amount
	bal.Buckets[toBucket] += amount
	bal.Version++
	return nil
}

func Get(s *state.EngineState, userID types.UserID, asset string, bucket int8) int64 {
	us, ok := s.Users[userID]
	if !ok {
		return 0
	}
	bal, ok := us.Balances[asset]
	if !ok {
		return 0
	}
	return bal.Get(bucket)
}
