package linear

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
	amount := safemath.Mul(int64(price), int64(absQty(qty)))
	inst := c.registry.GetInstrument(symbol)
	bal := c.users.GetBalance(userID, inst.QuoteAsset)
	return bal.Get(constants.BUCKET_AVAILABLE) >= amount
}

func (c *Clearing) Reserve(userID types.UserID, symbol string, qty types.Quantity, price types.Price) error {
	if qty == 0 {
		return nil
	}
	amount := safemath.Mul(int64(price), int64(absQty(qty)))
	inst := c.registry.GetInstrument(symbol)
	bal := c.users.GetBalance(userID, inst.QuoteAsset)
	if !bal.Move(constants.BUCKET_AVAILABLE, constants.BUCKET_LOCKED, amount) {
		return ErrInsufficientBalance
	}
	return nil
}

func (c *Clearing) Release(userID types.UserID, symbol string, qty types.Quantity, price types.Price) {
	if qty == 0 {
		return
	}
	amount := safemath.Mul(int64(price), int64(absQty(qty)))
	inst := c.registry.GetInstrument(symbol)
	bal := c.users.GetBalance(userID, inst.QuoteAsset)
	bal.Move(constants.BUCKET_LOCKED, constants.BUCKET_AVAILABLE, amount)
}

func (c *Clearing) ExecuteTrade(trade *types.Trade, taker *types.Order, maker *types.Order) {
	inst := c.registry.GetInstrument(trade.Symbol)
	price := int64(trade.Price)
	qty := int64(trade.Quantity)

	apply := func(order *types.Order, isMaker bool) {
		leverage := int64(order.Leverage)
		if leverage <= 0 {
			leverage = 1
		}
		bal := c.users.GetBalance(order.UserID, inst.QuoteAsset)
		if isMaker {
			lockedAmount := safemath.MulDiv(int64(order.Price), qty, leverage)
			bal.Move(constants.BUCKET_LOCKED, constants.BUCKET_AVAILABLE, lockedAmount)
		}
		marginAmount := safemath.MulDiv(price, qty, leverage)
		bal.Move(constants.BUCKET_AVAILABLE, constants.BUCKET_MARGIN, marginAmount)

		pos := c.users.GetPosition(order.UserID, order.Symbol)
		posSide := constants.SIDE_LONG
		if order.Side == constants.ORDER_SIDE_SELL {
			posSide = constants.SIDE_SHORT
		}
		pos.Update(types.Quantity(qty), types.Price(price), posSide, int8(leverage))
	}

	apply(taker, false)
	apply(maker, true)
}

func absQty(q types.Quantity) types.Quantity {
	if q < 0 {
		return -q
	}
	return q
}
