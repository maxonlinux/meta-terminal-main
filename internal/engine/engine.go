package engine

import (
	"github.com/maxonlinux/meta-terminal-go/internal/matching"
	"github.com/maxonlinux/meta-terminal-go/internal/oms"
	orderbook "github.com/maxonlinux/meta-terminal-go/internal/orderbook"
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

type OrderCallback interface {
	OnChildOrderCreated(order *types.Order)
}

type Engine struct {
	store    *oms.Service
	books    map[int8]map[string]*orderbook.OrderBook // Category + symbol orderbooks
	commands chan func()
	callback OrderCallback
	done     chan struct{}
}

func NewEngine(store *oms.Service, cb OrderCallback) *Engine {
	e := &Engine{
		store: store,
		books: map[int8]map[string]*orderbook.OrderBook{
			constants.CATEGORY_SPOT:   make(map[string]*orderbook.OrderBook),
			constants.CATEGORY_LINEAR: make(map[string]*orderbook.OrderBook),
		},
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

func (e *Engine) PlaceOrder(req *types.PlaceOrderRequest) error {
	result := make(chan error, 1)

	e.commands <- func() {
		if req.Quantity.Sign() <= 0 {
			result <- constants.ErrInvalidQuantity
			return
		}

		// Category validation keeps SPOT and LINEAR books isolated.
		if req.Category != constants.CATEGORY_SPOT && req.Category != constants.CATEGORY_LINEAR {
			result <- constants.ErrInvalidCategory
			return
		}

		if !req.TriggerPrice.IsZero() {
			if req.Category == constants.CATEGORY_SPOT {
				result <- constants.ErrConditionalSpot
				return
			}

			switch req.Side {
			case constants.ORDER_SIDE_BUY:
				if math.Cmp(req.TriggerPrice, req.Price) >= 0 {
					result <- constants.ErrInvalidTriggerForBuy
					return
				}
			case constants.ORDER_SIDE_SELL:
				if math.Cmp(req.TriggerPrice, req.Price) <= 0 {
					result <- constants.ErrInvalidTriggerForSell
					return
				}
			}
		}

		if req.ReduceOnly && req.Category == constants.CATEGORY_SPOT {
			result <- constants.ErrReduceOnlySpot
			return
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

		book, err := e.getBook(order.Category, order.Symbol)
		if err != nil {
			result <- err
			return
		}
		result <- matching.MatchOrder(order, book, e.applyTrade)
	}

	return <-result
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
