package spot

import (
	"errors"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/registry"
	"github.com/anomalyco/meta-terminal-go/internal/safemath"
	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

var ErrInsufficientBalance = errors.New("insufficient balance")

type Clearing struct {
	users    *state.Users
	registry *registry.Registry
}

func NewClearing(users *state.Users, reg *registry.Registry) *Clearing {
	return &Clearing{users: users, registry: reg}
}

func (c *Clearing) CanReserve(userID types.UserID, symbol string, qty types.Quantity, price types.Price) bool {
	if qty == 0 {
		return true
	}
	inst := c.registry.GetInstrument(symbol)
	if qty > 0 {
		amount := safemath.Mul(int64(price), int64(qty))
		bal := c.users.GetBalance(userID, inst.QuoteAsset)
		return bal.Get(constants.BUCKET_AVAILABLE) >= amount
	}
	amount := int64(-qty)
	bal := c.users.GetBalance(userID, inst.BaseAsset)
	return bal.Get(constants.BUCKET_AVAILABLE) >= amount
}

func (c *Clearing) Reserve(userID types.UserID, symbol string, qty types.Quantity, price types.Price) error {
	if qty == 0 {
		return nil
	}
	inst := c.registry.GetInstrument(symbol)
	if qty > 0 {
		amount := safemath.Mul(int64(price), int64(qty))
		bal := c.users.GetBalance(userID, inst.QuoteAsset)
		if !bal.Move(constants.BUCKET_AVAILABLE, constants.BUCKET_LOCKED, amount) {
			return ErrInsufficientBalance
		}
		return nil
	}
	amount := int64(-qty)
	bal := c.users.GetBalance(userID, inst.BaseAsset)
	if !bal.Move(constants.BUCKET_AVAILABLE, constants.BUCKET_LOCKED, amount) {
		return ErrInsufficientBalance
	}
	return nil
}

func (c *Clearing) Release(userID types.UserID, symbol string, qty types.Quantity, price types.Price) {
	if qty == 0 {
		return
	}
	inst := c.registry.GetInstrument(symbol)
	if qty > 0 {
		amount := safemath.Mul(int64(price), int64(qty))
		bal := c.users.GetBalance(userID, inst.QuoteAsset)
		bal.Move(constants.BUCKET_LOCKED, constants.BUCKET_AVAILABLE, amount)
		return
	}
	amount := int64(-qty)
	bal := c.users.GetBalance(userID, inst.BaseAsset)
	bal.Move(constants.BUCKET_LOCKED, constants.BUCKET_AVAILABLE, amount)
}

func (c *Clearing) ExecuteTrade(trade *types.Trade, taker *types.Order, maker *types.Order) {
	inst := c.registry.GetInstrument(trade.Symbol)
	price := int64(trade.Price)
	qty := int64(trade.Quantity)

	apply := func(order *types.Order, isMaker bool) {
		if order.Side == constants.ORDER_SIDE_BUY {
			quoteBal := c.users.GetBalance(order.UserID, inst.QuoteAsset)
			baseBal := c.users.GetBalance(order.UserID, inst.BaseAsset)
			if isMaker {
				lockedAmount := safemath.Mul(int64(order.Price), qty)
				quoteBal.Move(constants.BUCKET_LOCKED, constants.BUCKET_AVAILABLE, lockedAmount)
			}
			cost := safemath.Mul(price, qty)
			quoteBal.Deduct(constants.BUCKET_AVAILABLE, cost)
			baseBal.Add(constants.BUCKET_AVAILABLE, qty)
			return
		}

		baseBal := c.users.GetBalance(order.UserID, inst.BaseAsset)
		quoteBal := c.users.GetBalance(order.UserID, inst.QuoteAsset)
		if isMaker {
			baseBal.Move(constants.BUCKET_LOCKED, constants.BUCKET_AVAILABLE, qty)
		}
		baseBal.Deduct(constants.BUCKET_AVAILABLE, qty)
		proceeds := safemath.Mul(price, qty)
		quoteBal.Add(constants.BUCKET_AVAILABLE, proceeds)
	}

	apply(taker, false)
	apply(maker, true)
}

func (c *Clearing) BalanceSnapshot(userID types.UserID, asset string) [3]int64 {
	bal := c.users.GetBalance(userID, asset)
	return bal.Snapshot()
}

func (c *Clearing) BalanceRestore(userID types.UserID, asset string, buckets [3]int64) {
	bal := c.users.GetBalance(userID, asset)
	bal.Restore(buckets)
}
