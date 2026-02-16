package engine

import (
	"github.com/maxonlinux/meta-terminal-go/pkg/logging"
	"github.com/maxonlinux/meta-terminal-go/pkg/outbox"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
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

	result := e.executeOrder(order, book, writer, false)
	if result.Err != nil {
		logging.Log().Error().Int64("order", int64(order.ID)).Int64("user", int64(order.UserID)).Str("symbol", order.Symbol).Err(result.Err).Msg("engine conditional: activation failed")
	}
}
