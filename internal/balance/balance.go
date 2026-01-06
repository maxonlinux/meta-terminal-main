package balance

import (
	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/position"
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

func GetOrCreate(s *state.State, userID types.UserID, asset string) *types.UserBalance {
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
	return balance
}

func Transfer(s *state.State, fromUserID, toUserID types.UserID, asset string, amount int64) {
	fromUs := s.GetUserState(fromUserID)
	toUs := s.GetUserState(toUserID)

	fromBalance, ok := fromUs.Balances[asset]
	if !ok {
		fromBalance = &types.UserBalance{
			UserID:    fromUserID,
			Asset:     asset,
			Available: 0,
			Locked:    0,
			Margin:    0,
			Version:   0,
		}
		fromUs.Balances[asset] = fromBalance
	}

	toBalance, ok := toUs.Balances[asset]
	if !ok {
		toBalance = &types.UserBalance{
			UserID:    toUserID,
			Asset:     asset,
			Available: 0,
			Locked:    0,
			Margin:    0,
			Version:   0,
		}
		toUs.Balances[asset] = toBalance
	}

	fromBalance.Available -= amount
	fromBalance.Version++
	toBalance.Available += amount
	toBalance.Version++
}

func LockForOrder(s *state.State, category int8, userID types.UserID, order *types.Order, leverage int8) {
	if order.Type == constants.ORDER_TYPE_MARKET {
		return
	}

	bal := GetOrCreate(s, userID, "USDT")
	toLock := int64(order.Quantity-order.Filled) * int64(order.Price)

	if category == constants.CATEGORY_SPOT {
		bal.Locked += toLock
	} else {
		margin := position.CalculateMargin(order.Quantity-order.Filled, order.Price, leverage)
		bal.Margin += margin
		bal.Locked += margin
	}
}

func UnlockForOrder(s *state.State, category int8, userID types.UserID, order *types.Order, leverage int8) {
	bal := GetOrCreate(s, userID, "USDT")
	toUnlock := int64(order.Quantity-order.Filled) * int64(order.Price)

	if category == constants.CATEGORY_SPOT {
		bal.Locked -= toUnlock
	} else {
		margin := position.CalculateMargin(order.Quantity-order.Filled, order.Price, leverage)
		bal.Locked -= margin
	}

	if bal.Locked < 0 {
		bal.Locked = 0
	}
}

func AdjustLocked(s *state.State, category int8, userID types.UserID, order *types.Order, oldQty types.Quantity, oldPrice types.Price, leverage int8) {
	bal := GetOrCreate(s, userID, "USDT")
	oldQtyFloat := float64(oldQty)
	oldPriceFloat := float64(oldPrice)
	newQtyFloat := float64(order.Quantity)
	newPriceFloat := float64(order.Price)

	if category == constants.CATEGORY_SPOT {
		oldLocked := int64(oldQtyFloat * oldPriceFloat)
		newLocked := int64(newQtyFloat * newPriceFloat)
		bal.Locked += newLocked - oldLocked
	} else {
		oldMargin := position.CalculateMargin(oldQty, oldPrice, leverage)
		newMargin := position.CalculateMargin(order.Quantity, order.Price, leverage)
		bal.Locked += newMargin - oldMargin
	}
}
