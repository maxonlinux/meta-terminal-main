package matching

import (
	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/types"
)

type TradeCallback func(trade *types.Trade)

type Engine struct {
	category      int8
	orderStore    *state.OrderStore
	tradeCallback TradeCallback
}

func New(category int8, orderStore *state.OrderStore, callback TradeCallback) *Engine {
	return &Engine{
		category:      category,
		orderStore:    orderStore,
		tradeCallback: callback,
	}
}

func (e *Engine) ProcessOrder(order *types.Order) {
	switch order.TIF {
	case constants.TIF_GTC, constants.TIF_POST_ONLY:
		trades, remaining := e.match(order)
		order.Filled = order.Quantity - remaining

		if remaining > 0 {
			e.addToBook(order)
			if order.Filled == 0 {
				order.Status = constants.ORDER_STATUS_NEW
			} else {
				order.Status = constants.ORDER_STATUS_PARTIALLY_FILLED
			}
		} else if len(trades) > 0 {
			order.Status = constants.ORDER_STATUS_FILLED
		}

	case constants.TIF_IOC:
		trades, remaining := e.match(order)
		order.Filled = order.Quantity - remaining
		if remaining > 0 {
			order.Status = constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED
		} else if len(trades) > 0 {
			order.Status = constants.ORDER_STATUS_FILLED
		} else {
			order.Status = constants.ORDER_STATUS_CANCELED
		}

	case constants.TIF_FOK:
		trades, remaining := e.match(order)
		if remaining > 0 {
			for _, t := range trades {
				e.reverseTrade(t)
			}
			order.Status = constants.ORDER_STATUS_CANCELED
		} else {
			order.Filled = order.Quantity
			order.Status = constants.ORDER_STATUS_FILLED
		}
	}
}

func (e *Engine) match(order *types.Order) ([]*types.Trade, types.Quantity) {
	return nil, 0
}

func (e *Engine) reverseTrade(trade *types.Trade) {
	taker := e.orderStore.Get(trade.TakerOrderID)
	maker := e.orderStore.Get(trade.MakerOrderID)

	if taker != nil {
		taker.Filled -= trade.Quantity
	}
	if maker != nil {
		maker.Filled -= trade.Quantity
	}
}

func (e *Engine) addToBook(order *types.Order) {
}
