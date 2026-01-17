package matching

import (
	orderbook "github.com/maxonlinux/meta-terminal-go/internal/orderbook"
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/maxonlinux/meta-terminal-go/pkg/utils"
)

// MatchOrder executes the full order lifecycle, including TIF-specific behavior.
func MatchOrder(order *types.Order, book *orderbook.OrderBook, applyTrade orderbook.TradeHandler) error {
	if order.IsConditional {
		return nil
	}

	limitPrice := order.Price
	if order.Type == constants.ORDER_TYPE_MARKET {
		limitPrice = types.Price{}
	}

	// setStatus updates the order with consistent timestamps.
	setStatus := func(status int8) {
		now := utils.NowNano()
		order.Status = status
		order.UpdatedAt = now
	}

	// TIF-driven flow: each case handles its execution policy.
	switch order.TIF {
	case constants.TIF_POST_ONLY:
		// Post-only must rest on the book and cannot match immediately.
		if order.Type == constants.ORDER_TYPE_MARKET || book.WouldCross(order.Side, order.Price) {
			return constants.ErrPostOnlyWouldMatch
		}
		setStatus(constants.ORDER_STATUS_NEW)
		book.Add(order)
		return nil

	case constants.TIF_FOK:
		// Fill-or-kill must fully execute or reject before matching.
		needed := math.Sub(order.Quantity, order.Filled)
		if needed.Sign() > 0 && book.AvailableQuantity(order.Side, limitPrice, needed).Cmp(needed) < 0 {
			return constants.ErrFOKInsufficientLiquidity
		}
		book.Match(order, limitPrice, applyTrade)
		if math.Cmp(order.Filled, order.Quantity) != 0 {
			panic("FOK order partially filled")
		}
		setStatus(constants.ORDER_STATUS_FILLED)
		return nil

	case constants.TIF_IOC:
		// Immediate-or-cancel never rests on the book.
		book.Match(order, limitPrice, applyTrade)
		isFilled := math.Cmp(order.Filled, order.Quantity) == 0
		if order.Filled.IsZero() {
			setStatus(constants.ORDER_STATUS_CANCELED)
			return nil
		}
		if !isFilled {
			setStatus(constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED)
			return nil
		}
		setStatus(constants.ORDER_STATUS_FILLED)
		return nil

	case constants.TIF_GTC:
		// GTC rests any remaining quantity after matching.
		book.Match(order, limitPrice, applyTrade)
		isFilled := math.Cmp(order.Filled, order.Quantity) == 0
		if isFilled {
			setStatus(constants.ORDER_STATUS_FILLED)
			return nil
		}

		status := int8(constants.ORDER_STATUS_PARTIALLY_FILLED)
		if order.Filled.IsZero() {
			status = constants.ORDER_STATUS_NEW
		}

		setStatus(status)
		book.Add(order)
		return nil
	default:
		// Reject unsupported time-in-force values immediately.
		return constants.ErrInvalidTIF
	}
}
