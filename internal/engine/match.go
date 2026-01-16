package engine

import (
	orderbook "github.com/maxonlinux/meta-terminal-go/internal/orderbook"
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/maxonlinux/meta-terminal-go/pkg/utils"
)

// Match executes taker matching against the orderbook and applies resulting trades.
// The engine owns trade side-effects (status updates and balance moves).
func (e *Engine) Match(order *types.Order) (bool, error) {
	limitPrice := e.matchLimitPrice(order)
	matched := false

	book, err := e.bookFor(order.Category, order.Symbol)
	if err != nil {
		return false, err
	}

	// The orderbook performs price-time matching and emits trades.
	book.Match(order, limitPrice, func(trade types.Trade) {
		matched = true
		e.applyTrade(trade)
	})

	return matched, nil
}

// checkWouldMatch determines whether a post-only order would immediately cross.
func (e *Engine) checkWouldMatch(order *types.Order) (bool, error) {
	if order.Type == constants.ORDER_TYPE_MARKET {
		// Market orders always cross if any liquidity is present.
		return true, nil
	}
	book, err := e.bookFor(order.Category, order.Symbol)
	if err != nil {
		return false, err
	}
	return book.WouldCross(order.Side, order.Price), nil
}

// checkFullLiquidity verifies if the book can fully satisfy a FOK order.
func (e *Engine) checkFullLiquidity(order *types.Order) (bool, error) {
	needed := remaining(order)
	if needed.Sign() <= 0 {
		return true, nil
	}
	limitPrice := e.matchLimitPrice(order)
	book, err := e.bookFor(order.Category, order.Symbol)
	if err != nil {
		return false, err
	}
	available := book.AvailableQuantity(order.Side, limitPrice, needed)
	return available.Cmp(needed) >= 0, nil
}

// applyTrade updates maker order status and triggers balance movements.
func (e *Engine) applyTrade(trade types.Trade) {
	// Maker order state is updated here because matching mutates filled amounts only.
	if trade.MakerOrder != nil {
		now := utils.NowNano()
		if remaining(trade.MakerOrder).Sign() == 0 {
			trade.MakerOrder.Status = constants.ORDER_STATUS_FILLED
			trade.MakerOrder.ClosedAt = now
			trade.MakerOrder.UpdatedAt = now
		} else {
			trade.MakerOrder.Status = constants.ORDER_STATUS_PARTIALLY_FILLED
			trade.MakerOrder.UpdatedAt = now
		}
	}

	// TODO: Apply balance transfers here using BUSINESS_RULES reserve formulas.
}

// matchLimitPrice determines the taker price constraint.
func (e *Engine) matchLimitPrice(order *types.Order) types.Price {
	if order.Type == constants.ORDER_TYPE_MARKET {
		// Market orders accept any price; we skip price checks in the matcher.
		return types.Price{}
	}
	return order.Price
}

// bookFor returns the orderbook for the provided market category and symbol.
func (e *Engine) bookFor(category int8, symbol string) (*orderbook.OrderBook, error) {
	// Unknown categories are rejected to keep SPOT and LINEAR isolated.
	bookSet, ok := e.books[category]
	if !ok {
		return nil, constants.ErrInvalidCategory
	}
	if book, ok := bookSet[symbol]; ok {
		return book, nil
	}

	// Lazy initialization keeps empty symbols out of memory until needed.
	book := orderbook.New()
	bookSet[symbol] = book
	return book, nil
}

// remaining computes unfilled quantity for an order.
func remaining(order *types.Order) types.Quantity {
	// Remaining is derived from the canonical order fields to keep one source of truth.
	return math.Sub(order.Quantity, order.Filled)
}
