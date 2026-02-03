package engine

import (
	"github.com/maxonlinux/meta-terminal-go/internal/clearing"
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/events"
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/outbox"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

func oppositeSideForPosition(size types.Quantity) int8 {
	if size.Sign() > 0 {
		return constants.ORDER_SIDE_SELL
	}
	return constants.ORDER_SIDE_BUY
}

func (e *Engine) checkLiquidations(symbol string, price types.Price, writer outbox.Writer) {
	for userID, positions := range e.portfolio.Positions {
		pos := positions[symbol]
		if pos == nil || math.Sign(pos.Size) == 0 {
			continue
		}
		liqPrice := clearing.LiquidationPrice(pos.EntryPrice, pos.Leverage, pos.Size)
		pos.LiqPrice = liqPrice
		if !clearing.ShouldLiquidate(price, liqPrice, pos.Size) {
			continue
		}
		if e.publisher != nil {
			e.publisher.OnLiquidation(LiquidationEvent{
				UserID: userID,
				Symbol: symbol,
				Stage:  "MARGIN_CALL",
				Price:  price,
				Size:   pos.Size,
			})
		}
		e.liquidatePosition(userID, pos, writer)
	}
}

func (e *Engine) liquidatePosition(userID types.UserID, pos *types.Position, writer outbox.Writer) {
	if pos == nil {
		return
	}
	order := e.store.Create(
		userID,
		pos.Symbol,
		constants.CATEGORY_LINEAR,
		constants.ORDER_ORIGIN_SYSTEM,
		oppositeSideForPosition(pos.Size),
		constants.ORDER_TYPE_MARKET,
		constants.TIF_IOC,
		types.Price{},
		math.AbsFixed(pos.Size),
		types.Price{},
		true,
		true,
		constants.STOP_ORDER_TYPE_STOP,
	)
	if writer != nil {
		_ = writer.Record(events.EncodeOrderPlaced(order))
	}
	if e.publisher != nil {
		e.publisher.OnLiquidation(LiquidationEvent{
			UserID: userID,
			Symbol: pos.Symbol,
			Stage:  "LIQUIDATION_STARTED",
			Price:  e.lastPrices[pos.Symbol],
			Size:   pos.Size,
		})
		e.publisher.OnOrderUpdated(order)
	}

	e.activateConditional(order, writer)

	if e.publisher != nil {
		e.publisher.OnLiquidation(LiquidationEvent{
			UserID: userID,
			Symbol: pos.Symbol,
			Stage:  "LIQUIDATED",
			Price:  e.lastPrices[pos.Symbol],
			Size:   pos.Size,
		})
	}
}
