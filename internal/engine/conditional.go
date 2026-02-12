package engine

import (
	"log"

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
		log.Printf("engine conditional: activation failed order=%d user=%d symbol=%s err=%v", order.ID, order.UserID, order.Symbol, result.Err)
	}
}
