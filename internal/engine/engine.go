package engine

import (
	"errors"

	"github.com/maxonlinux/meta-terminal-go/internal/oms"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

// Engine — главный оркестратор торговой системы
// Единая точка входа для всех операций
type Engine struct {
	store *oms.OrderStore
}

// NewEngine создает движок
func NewEngine(store *oms.OrderStore) *Engine {
	return &Engine{store: store}
}

// PlaceOrder размещает ордер
func (e *Engine) PlaceOrder(req PlaceOrderRequest) (*PlaceOrderResult, error) {
	order := e.store.Create(
		req.UserID,
		req.Symbol,
		req.Category,
		req.Side,
		req.Type,
		req.TIF,
		req.Price,
		req.Quantity,
		req.TriggerPrice,
		req.ReduceOnly,
		req.CloseOnTrigger,
		req.StopOrderType,
	)

	return &PlaceOrderResult{Order: order}, nil
}

// CancelOrder отменяет ордер
func (e *Engine) CancelOrder(orderID types.OrderID) error {
	_, ok := e.store.Get(orderID)
	if !ok {
		return ErrOrderNotFound
	}

	e.store.CancelOrder(orderID)
	return nil
}

// AmendOrder уменьшает количество ордера
func (e *Engine) AmendOrder(orderID types.OrderID, reduceBy types.Quantity) {
	e.store.AmendOrder(orderID, reduceBy)
}

// OnPositionChange вызывается при изменении позиции
// Trimming reduce-only ордера если нужно
func (e *Engine) OnPositionChange(userID types.UserID, symbol string, positionSize types.Quantity) {
	if positionSize == 0 {
		return
	}

	h := e.store.GetROHeap(symbol, positionSize)
	if h == nil || h.Len() == 0 {
		return
	}

	total := e.store.GetROSumCap(symbol, userID)
	if total <= positionSize {
		return
	}

	excess := total - positionSize

	for excess > 0 && h.Len() > 0 {
		o := h.Peek()
		if o == nil {
			break
		}

		remaining := o.Quantity - o.Filled
		if remaining <= excess {
			// Full cancel
			e.store.CancelOrder(o.ID)
			excess -= remaining
		} else {
			// Partial amend
			e.store.AmendOrder(o.ID, excess)
			excess = 0
		}
	}
}

// OnFill вызывается при исполнении ордера
func (e *Engine) OnFill(orderID types.OrderID, filledQty types.Quantity) {
	e.store.FillOrder(orderID, filledQty)
}

// PlaceOrderRequest — входящий запрос
type PlaceOrderRequest struct {
	UserID         types.UserID
	Symbol         string
	Category       int8
	Side           int8
	Type           int8
	TIF            int8
	Price          types.Price
	Quantity       types.Quantity
	TriggerPrice   types.Price
	ReduceOnly     bool
	CloseOnTrigger bool
	StopOrderType  int8
}

// PlaceOrderResult — результат
type PlaceOrderResult struct {
	Order *types.Order
}

var ErrOrderNotFound = errors.New("order not found")
