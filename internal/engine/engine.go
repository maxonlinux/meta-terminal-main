package engine

import (
	"github.com/maxonlinux/meta-terminal-go/internal/oms"
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/maxonlinux/meta-terminal-go/pkg/utils"
)

type OrderCallback interface {
	OnChildOrderCreated(order *types.Order)
}

type Engine struct {
	store    *oms.Service
	commands chan func()
	callback OrderCallback
	done     chan struct{}
}

func NewEngine(store *oms.Service, cb OrderCallback) *Engine {
	e := &Engine{
		store:    store,
		commands: make(chan func(), 1000),
		callback: cb,
		done:     make(chan struct{}),
	}

	go func() {
		for {
			select {
			case cmd := <-e.commands:
				cmd()
			case <-e.done:
				return
			}
		}
	}()

	return e
}

func (e *Engine) Shutdown() {
	close(e.done)
}

func (e *Engine) Execute(cmd func()) {
	e.commands <- cmd
}

func (e *Engine) PlaceOrder(req *PlaceOrderRequest) error {
	result := make(chan error, 1)

	e.commands <- func() {
		err := e.placeOrderImpl(req)
		result <- err
	}

	return <-result
}

func (e *Engine) placeOrderImpl(req *PlaceOrderRequest) error {
	if err := e.validatePlaceOrder(req); err != nil {
		return err
	}

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

	switch req.TIF {
	case constants.TIF_GTC:
		return e.handleGTC(order)
	case constants.TIF_IOC:
		return e.handleIOC(order)
	case constants.TIF_FOK:
		return e.handleFOK(order)
	case constants.TIF_POST_ONLY:
		return e.handlePostOnly(order)
	}

	return nil
}

func (e *Engine) validatePlaceOrder(req *PlaceOrderRequest) error {
	if req.Quantity.Sign() <= 0 {
		return constants.ErrInsufficientBalance
	}

	if !req.TriggerPrice.IsZero() {
		if req.Category == constants.CATEGORY_SPOT {
			return constants.ErrConditionalSpot
		}

		switch req.Side {
		case constants.ORDER_SIDE_BUY:
			if math.Cmp(req.TriggerPrice, req.Price) >= 0 {
				return constants.ErrInvalidTriggerForBuy
			}
		case constants.ORDER_SIDE_SELL:
			if math.Cmp(req.TriggerPrice, req.Price) <= 0 {
				return constants.ErrInvalidTriggerForSell
			}
		}
	}

	if req.ReduceOnly && req.Category == constants.CATEGORY_SPOT {
		return constants.ErrReduceOnlySpot
	}

	return nil
}

func (e *Engine) handleGTC(order *types.Order) error {
	if order.IsConditional {
		return nil
	}

	matched, err := e.matchAsTaker(order)
	if err != nil {
		return err
	}

	if matched {
		if math.Cmp(order.Filled, order.Quantity) == 0 {
			order.Status = constants.ORDER_STATUS_FILLED
			order.ClosedAt = utils.NowNano()
			return nil
		}
		order.Status = constants.ORDER_STATUS_PARTIALLY_FILLED
	}

	return nil
}

func (e *Engine) handleIOC(order *types.Order) error {
	if order.IsConditional {
		return nil
	}

	matched, err := e.matchAsTaker(order)
	if err != nil {
		return err
	}

	if !matched || order.Filled.IsZero() {
		order.Status = constants.ORDER_STATUS_CANCELED
		order.ClosedAt = utils.NowNano()
		return nil
	}

	if math.Cmp(order.Filled, order.Quantity) < 0 {
		order.Status = constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED
		order.ClosedAt = utils.NowNano()
	}

	return nil
}

func (e *Engine) handleFOK(order *types.Order) error {
	if order.IsConditional {
		return nil
	}

	canFullyFill, err := e.checkFullLiquidity(order)
	if err != nil {
		return err
	}

	if !canFullyFill {
		return constants.ErrFOKInsufficientLiquidity
	}

	_, err = e.matchAsTaker(order)
	if err != nil {
		return err
	}

	order.Status = constants.ORDER_STATUS_FILLED
	order.ClosedAt = utils.NowNano()

	return nil
}

func (e *Engine) handlePostOnly(order *types.Order) error {
	if order.IsConditional {
		return nil
	}

	wouldMatch, err := e.checkWouldMatch(order)
	if err != nil {
		return err
	}

	if wouldMatch {
		return constants.ErrPostOnlyWouldMatch
	}

	return nil
}

func (e *Engine) matchAsTaker(order *types.Order) (matched bool, err error) {
	return false, nil
}

func (e *Engine) checkFullLiquidity(order *types.Order) (bool, error) {
	return false, nil
}

func (e *Engine) checkWouldMatch(order *types.Order) (bool, error) {
	return false, nil
}

func (e *Engine) CancelOrder(id types.OrderID) error {
	result := make(chan error, 1)

	e.commands <- func() {
		err := e.store.Cancel(id)
		result <- err
	}

	return <-result
}

func (e *Engine) AmendOrder(id types.OrderID, newQty types.Quantity) error {
	result := make(chan error, 1)

	e.commands <- func() {
		err := e.store.Amend(id, newQty)
		result <- err
	}

	return <-result
}

func (e *Engine) FillOrder(id types.OrderID, qty types.Quantity) error {
	result := make(chan error, 1)

	e.commands <- func() {
		err := e.store.Fill(id, qty)
		result <- err
	}

	return <-result
}

func (e *Engine) OnPositionReduce(userID types.UserID, symbol string, size types.Quantity) {
	result := make(chan struct{}, 1)

	e.commands <- func() {
		e.store.OnPositionReduce(userID, symbol, size)
		result <- struct{}{}
	}

	<-result
}

func (e *Engine) OnPriceTick(symbol string, price types.Price) {
	result := make(chan struct{}, 1)

	e.commands <- func() {
		e.store.OnPriceTick(symbol, price, func(order *types.Order) {
			if e.callback != nil {
				e.callback.OnChildOrderCreated(order)
			}
		})
		result <- struct{}{}
	}

	<-result
}

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
