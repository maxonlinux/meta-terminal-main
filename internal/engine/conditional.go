package engine

import (
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/events"
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/outbox"
	"github.com/maxonlinux/meta-terminal-go/pkg/snowflake"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/maxonlinux/meta-terminal-go/pkg/utils"
)

func (e *Engine) activateConditional(order *types.Order, writer outbox.Writer) {
	if order == nil {
		return
	}
	book, err := e.getBook(order.Category, order.Symbol)
	if err != nil {
		return
	}

	order.IsConditional = false
	order.TriggerPrice = types.Price{}

	limitPrice := order.Price
	if order.Type == constants.ORDER_TYPE_MARKET {
		limitPrice = types.Price{}
	}

	var buf [8]types.Match
	matches := book.GetMatches(order, limitPrice, buf[:0])
	var matchQty types.Quantity
	var matchNotional types.Quantity
	for i := range matches {
		matchQty = math.Add(matchQty, matches[i].Quantity)
		matchNotional = math.Add(matchNotional, math.Mul(matches[i].Price, matches[i].Quantity))
	}
	remaining := math.Sub(order.Quantity, matchQty)

	if order.TIF == constants.TIF_FOK && math.Sign(remaining) > 0 {
		_ = e.store.Cancel(order.UserID, order.ID)
		if writer != nil {
			_ = writer.Record(events.EncodeOrderCanceled(events.OrderCanceledEvent{UserID: order.UserID, OrderID: order.ID, Timestamp: order.UpdatedAt}))
		}
		return
	}

	if math.Sign(matchQty) > 0 {
		avgPrice := types.Price(math.Div(matchNotional, matchQty))
		if err := e.clearing.Reserve(order.UserID, order.Symbol, order.Category, order.Side, matchQty, avgPrice); err != nil {
			return
		}
	}

	if math.Sign(remaining) > 0 && (order.TIF == constants.TIF_POST_ONLY || order.TIF == constants.TIF_GTC) {
		if err := e.clearing.Reserve(order.UserID, order.Symbol, order.Category, order.Side, remaining, order.Price); err != nil {
			return
		}
	}

	for i := range matches {
		match := matches[i]
		match.ID = types.TradeID(snowflake.Next())
		match.Timestamp = utils.NowNano()
		e.applyTrade(book, match, writer)
	}

	if order.TIF == constants.TIF_IOC || order.Type == constants.ORDER_TYPE_MARKET {
		if math.Cmp(order.Filled, order.Quantity) != 0 {
			_ = e.store.Cancel(order.UserID, order.ID)
			if writer != nil {
				_ = writer.Record(events.EncodeOrderCanceled(events.OrderCanceledEvent{UserID: order.UserID, OrderID: order.ID, Timestamp: order.UpdatedAt}))
			}
		}
		return
	}

	if order.TIF == constants.TIF_POST_ONLY {
		book.Add(order)
		if e.publisher != nil {
			e.publisher.OnOrderbookUpdated(order.Category, order.Symbol)
		}
		return
	}

	if math.Sign(remaining) > 0 {
		book.Add(order)
		if e.publisher != nil {
			e.publisher.OnOrderbookUpdated(order.Category, order.Symbol)
		}
	}
}
