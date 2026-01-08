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

func (s *Spot) Reset() {
	s.orderBooks = make(map[string]*orderbook.OrderBook)
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
		return nil, nil
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

	cost := int64(order.Quantity) * int64(order.Price)

	if order.Side == constants.ORDER_SIDE_BUY {
		if cost > 0 {
			if err := balance.Deduct(s.state, order.UserID, "USDT", types.BUCKET_AVAILABLE, cost); err != nil {
				pool.PutOrder(order)
				return nil, err
			}
			if err := balance.Add(s.state, order.UserID, "USDT", types.BUCKET_LOCKED, cost); err != nil {
				balance.Add(s.state, order.UserID, "USDT", types.BUCKET_AVAILABLE, cost)
				pool.PutOrder(order)
				return nil, err
			}
		}
	} else {
		if err := balance.Deduct(s.state, order.UserID, "BTC", types.BUCKET_AVAILABLE, int64(order.Quantity)); err != nil {
			pool.PutOrder(order)
			return nil, err
		}
		if err := balance.Add(s.state, order.UserID, "BTC", types.BUCKET_LOCKED, int64(order.Quantity)); err != nil {
			balance.Add(s.state, order.UserID, "BTC", types.BUCKET_AVAILABLE, int64(order.Quantity))
			pool.PutOrder(order)
			return nil, err
		}
	}

	trades, err := ob.AddOrder(order)
	if err != nil {
		pool.PutOrder(order)
		return nil, err
	}
	result.Trades = trades

	s.executeSpotTrades(trades, order)

	switch order.TIF {
	case constants.TIF_GTC, constants.TIF_POST_ONLY:
		if order.Filled == 0 {
			order.Status = constants.ORDER_STATUS_NEW
			result.Status = constants.ORDER_STATUS_NEW
		} else if order.Filled == order.Quantity {
			order.Status = constants.ORDER_STATUS_FILLED
			result.Status = constants.ORDER_STATUS_FILLED
		} else {
			order.Status = constants.ORDER_STATUS_PARTIALLY_FILLED
			result.Status = constants.ORDER_STATUS_PARTIALLY_FILLED
		}

	case constants.TIF_IOC:
		if order.Filled == 0 {
			order.Status = constants.ORDER_STATUS_CANCELED
			result.Status = constants.ORDER_STATUS_CANCELED
			s.refundUnfilled(order)
		} else if order.Filled < order.Quantity {
			order.Status = constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED
			result.Status = constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED
		} else {
			order.Status = constants.ORDER_STATUS_FILLED
			result.Status = constants.ORDER_STATUS_FILLED
		}

	case constants.TIF_FOK:
		if order.Filled == order.Quantity {
			order.Status = constants.ORDER_STATUS_FILLED
			result.Status = constants.ORDER_STATUS_FILLED
		} else {
			order.Status = constants.ORDER_STATUS_CANCELED
			result.Status = constants.ORDER_STATUS_CANCELED
			s.refundUnfilled(order)
		}
	}

	result.Filled = order.Filled
	result.Remaining = order.Quantity - order.Filled

	return result, nil
}

func (s *Spot) refundUnfilled(order *types.Order) {
	unfilledQty := order.Quantity - order.Filled
	orderCost := int64(unfilledQty) * int64(order.Price)

	if order.Side == constants.ORDER_SIDE_BUY {
		if orderCost > 0 {
			balance.Move(s.state, order.UserID, "USDT", types.BUCKET_LOCKED, types.BUCKET_AVAILABLE, orderCost)
		}
	} else {
		balance.Move(s.state, order.UserID, "BTC", types.BUCKET_LOCKED, types.BUCKET_AVAILABLE, int64(unfilledQty))
	}
}

func (s *Spot) executeSpotTrades(trades []*types.Trade, taker *types.Order) {
	for _, trade := range trades {
		maker := s.orderStore.Get(trade.MakerOrderID)
		if maker == nil {
			continue
		}

		trade.TakerID = taker.UserID
		trade.MakerID = maker.UserID
		trade.Symbol = taker.Symbol
		trade.ExecutedAt = types.NanoTime()

		tradeQty := int64(trade.Quantity)
		tradeCost := tradeQty * int64(trade.Price)

		orderPrice := int64(maker.Price)
		makerReserved := orderPrice * tradeQty

		if taker.Side == constants.ORDER_SIDE_BUY {
			balance.Move(s.state, taker.UserID, "USDT", types.BUCKET_LOCKED, types.BUCKET_AVAILABLE, makerReserved)
			balance.Deduct(s.state, taker.UserID, "USDT", types.BUCKET_AVAILABLE, tradeCost)
			balance.Add(s.state, maker.UserID, "USDT", types.BUCKET_AVAILABLE, tradeCost)

			balance.Move(s.state, maker.UserID, "BTC", types.BUCKET_LOCKED, types.BUCKET_AVAILABLE, tradeQty)
			balance.Deduct(s.state, maker.UserID, "BTC", types.BUCKET_AVAILABLE, tradeQty)
			balance.Add(s.state, taker.UserID, "BTC", types.BUCKET_AVAILABLE, tradeQty)
		} else {
			balance.Move(s.state, maker.UserID, "USDT", types.BUCKET_LOCKED, types.BUCKET_AVAILABLE, makerReserved)
			balance.Deduct(s.state, maker.UserID, "USDT", types.BUCKET_AVAILABLE, tradeCost)
			balance.Add(s.state, taker.UserID, "USDT", types.BUCKET_AVAILABLE, tradeCost)

			balance.Move(s.state, taker.UserID, "BTC", types.BUCKET_LOCKED, types.BUCKET_AVAILABLE, tradeQty)
			balance.Deduct(s.state, taker.UserID, "BTC", types.BUCKET_AVAILABLE, tradeQty)
			balance.Add(s.state, maker.UserID, "BTC", types.BUCKET_AVAILABLE, tradeQty)
		}
	}
}

func (s *Spot) CancelOrder(orderID types.OrderID, userID types.UserID) error {
	order := s.orderStore.Get(orderID)
	if order == nil {
		return nil
	}
	if userID != 0 && order.UserID != userID {
		return nil
	}

	if order.Status == constants.ORDER_STATUS_NEW || order.Status == constants.ORDER_STATUS_PARTIALLY_FILLED {
		s.refundUnfilled(order)

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
