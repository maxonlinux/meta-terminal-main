package spot

import (
	"github.com/anomalyco/meta-terminal-go/internal/balance"
	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/orderbook"
	"github.com/anomalyco/meta-terminal-go/internal/pool"
	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/types"
)

type Spot struct {
	state      *state.EngineState
	orderStore *state.OrderStore
	orderBooks map[string]*orderbook.OrderBook
}

func New(s *state.EngineState, orderStore *state.OrderStore) *Spot {
	return &Spot{
		state:      s,
		orderStore: orderStore,
		orderBooks: make(map[string]*orderbook.OrderBook),
	}
}

func (s *Spot) getOrderBook(symbol string) *orderbook.OrderBook {
	ob, ok := s.orderBooks[symbol]
	if !ok {
		ob = orderbook.New(constants.CATEGORY_SPOT, s.orderStore)
		s.orderBooks[symbol] = ob
	}
	return ob
}

func (s *Spot) PlaceOrder(input *types.OrderInput) (*types.OrderResult, error) {
	if input.ReduceOnly {
		return nil, nil
	}
	if input.CloseOnTrigger {
		input.CloseOnTrigger = false
	}

	orderID := pool.NextOrderID()
	order := pool.GetOrder()
	order.ID = orderID
	order.UserID = input.UserID
	order.Symbol = input.Symbol
	order.Side = input.Side
	order.Type = input.Type
	order.TIF = input.TIF
	order.Status = constants.ORDER_STATUS_NEW
	order.Price = input.Price
	order.Quantity = input.Quantity
	order.Filled = 0
	order.ReduceOnly = false
	order.CloseOnTrigger = false
	order.CreatedAt = types.NanoTime()
	order.UpdatedAt = order.CreatedAt

	result := pool.GetOrderResult()
	result.Order = order
	result.Trades = nil

	if input.TriggerPrice > 0 {
		order.Status = constants.ORDER_STATUS_UNTRIGGERED
		result.Status = constants.ORDER_STATUS_UNTRIGGERED
		return result, nil
	}

	ob := s.getOrderBook(input.Symbol)
	ob.AddOrder(order)

	switch order.TIF {
	case constants.TIF_GTC, constants.TIF_POST_ONLY:
		if order.Filled == 0 {
			order.Status = constants.ORDER_STATUS_NEW
			result.Status = constants.ORDER_STATUS_NEW
		} else {
			order.Status = constants.ORDER_STATUS_FILLED
			result.Status = constants.ORDER_STATUS_FILLED
			result.Filled = order.Filled
			result.Remaining = order.Quantity - order.Filled
		}

	case constants.TIF_IOC:
		if order.Filled == 0 {
			order.Status = constants.ORDER_STATUS_CANCELED
			result.Status = constants.ORDER_STATUS_CANCELED
		} else if order.Filled < order.Quantity {
			order.Status = constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED
			result.Status = constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED
		} else {
			order.Status = constants.ORDER_STATUS_FILLED
			result.Status = constants.ORDER_STATUS_FILLED
		}
		result.Filled = order.Filled
		result.Remaining = order.Quantity - order.Filled

	case constants.TIF_FOK:
		if order.Filled == order.Quantity {
			order.Status = constants.ORDER_STATUS_FILLED
			result.Status = constants.ORDER_STATUS_FILLED
			result.Filled = order.Filled
			result.Remaining = 0
		} else {
			order.Status = constants.ORDER_STATUS_CANCELED
			result.Status = constants.ORDER_STATUS_CANCELED
			result.Filled = 0
			result.Remaining = order.Quantity
		}
	}

	return result, nil
}

func (s *Spot) CancelOrder(orderID types.OrderID, userID types.UserID) error {
	order := s.orderStore.Get(orderID)
	if order == nil {
		return nil
	}
	if userID != 0 && order.UserID != userID {
		return nil
	}

	if order.Status == constants.ORDER_STATUS_NEW {
		orderValue := int64(order.Quantity-order.Filled) * int64(order.Price)
		balance.Move(s.state, order.UserID, "USDT", types.BUCKET_LOCKED, types.BUCKET_AVAILABLE, orderValue)

		ob := s.getOrderBook(order.Symbol)
		ob.RemoveOrder(order)
	}

	order.Status = constants.ORDER_STATUS_CANCELED
	s.orderStore.Remove(orderID)
	pool.PutOrder(order)
	return nil
}

func (s *Spot) GetOrderBook(symbol string, limit int) (bids, asks []int64) {
	ob := s.orderBooks[symbol]
	if ob == nil {
		return nil, nil
	}
	bids = ob.GetDepth(constants.ORDER_SIDE_BUY, limit)
	asks = ob.GetDepth(constants.ORDER_SIDE_SELL, limit)
	return bids, asks
}
