package engine

import (
	"time"

	"github.com/maxonlinux/meta-terminal-go/internal/clearing"
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/events"
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/outbox"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

const liquidationRetryDelay = 5 * time.Second

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
		if !e.allowLiquidation(userID, symbol) {
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
		constants.TRIGGER_DIRECTION_NONE,
	)
	if writer != nil {
		_ = writer.Record(events.EncodeOrderPlaced(events.OrderPlacedEvent{
			Order:      order,
			Instrument: e.registry.GetInstrument(order.Symbol),
		}))
	}
	if e.publisher != nil {
		price := types.Price{}
		if tick, ok := e.registry.GetPrice(pos.Symbol); ok {
			price = tick.Price
		}
		e.publisher.OnLiquidation(LiquidationEvent{
			UserID: userID,
			Symbol: pos.Symbol,
			Stage:  "LIQUIDATION_STARTED",
			Price:  price,
			Size:   pos.Size,
		})
		e.publisher.OnOrderUpdated(order)
	}

	order.IsConditional = false
	order.TriggerPrice = types.Price{}
	book, err := e.getBook(constants.CATEGORY_LINEAR, pos.Symbol)
	if err != nil {
		return
	}
	res := e.executeOrder(order, book, writer, false)
	if res.Err != nil {
		return
	}
	remaining := math.Sub(order.Quantity, order.Filled)
	if math.Sign(remaining) > 0 {
		e.scheduleLiquidationRetry(userID, pos.Symbol)
	} else {
		e.clearLiquidationRetry(userID, pos.Symbol)
	}

	if e.publisher != nil {
		price := types.Price{}
		if tick, ok := e.registry.GetPrice(pos.Symbol); ok {
			price = tick.Price
		}
		e.publisher.OnLiquidation(LiquidationEvent{
			UserID: userID,
			Symbol: pos.Symbol,
			Stage:  "LIQUIDATED",
			Price:  price,
			Size:   pos.Size,
		})
	}
}

func (e *Engine) allowLiquidation(userID types.UserID, symbol string) bool {
	key := bookLockKey{symbol: symbol, category: constants.CATEGORY_LINEAR}
	now := time.Now()
	e.liqMu.Lock()
	next, ok := e.liqNext[key]
	if ok && now.Before(next) {
		e.liqMu.Unlock()
		return false
	}
	e.liqMu.Unlock()
	return true
}

func (e *Engine) scheduleLiquidationRetry(userID types.UserID, symbol string) {
	key := bookLockKey{symbol: symbol, category: constants.CATEGORY_LINEAR}
	next := time.Now().Add(liquidationRetryDelay)
	e.liqMu.Lock()
	e.liqNext[key] = next
	e.liqMu.Unlock()

	time.AfterFunc(liquidationRetryDelay, func() {
		pos := e.portfolio.GetPosition(userID, symbol)
		if pos == nil || math.Sign(pos.Size) == 0 {
			e.clearLiquidationRetry(userID, symbol)
			return
		}
		e.withBookLock(symbol, constants.CATEGORY_LINEAR, func() CommandResult {
			e.liquidatePosition(userID, pos, nil)
			return CommandResult{}
		})
	})
}

func (e *Engine) clearLiquidationRetry(userID types.UserID, symbol string) {
	key := bookLockKey{symbol: symbol, category: constants.CATEGORY_LINEAR}
	e.liqMu.Lock()
	delete(e.liqNext, key)
	e.liqMu.Unlock()
}
