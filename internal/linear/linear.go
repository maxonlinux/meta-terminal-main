package linear

import (
	"github.com/anomalyco/meta-terminal-go/internal/balance"
	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/orderbook"
	"github.com/anomalyco/meta-terminal-go/internal/pool"
	"github.com/anomalyco/meta-terminal-go/internal/position"
	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/internal/trigger"
	"github.com/anomalyco/meta-terminal-go/types"
)

type Linear struct {
	state      *state.EngineState
	orderStore *state.OrderStore
	orderBooks map[string]*orderbook.OrderBook
	triggerMon map[string]*trigger.Monitor
}

func New(s *state.EngineState, orderStore *state.OrderStore) *Linear {
	return &Linear{
		state:      s,
		orderStore: orderStore,
		orderBooks: make(map[string]*orderbook.OrderBook),
		triggerMon: make(map[string]*trigger.Monitor),
	}
}

func (l *Linear) getOrderBook(symbol string) *orderbook.OrderBook {
	ob, ok := l.orderBooks[symbol]
	if !ok {
		ob = orderbook.New(constants.CATEGORY_LINEAR, l.orderStore)
		l.orderBooks[symbol] = ob
	}
	return ob
}

func (l *Linear) getTriggerMonitor(symbol string) *trigger.Monitor {
	mon, ok := l.triggerMon[symbol]
	if !ok {
		mon = trigger.NewMonitor()
		l.triggerMon[symbol] = mon
	}
	return mon
}

func (l *Linear) PlaceOrder(input *types.OrderInput) (*types.OrderResult, error) {
	if input.ReduceOnly {
		if err := position.ReduceOnlyValidate(l.state, input.UserID, input.Symbol, input.Quantity, input.Side); err != nil {
			return nil, err
		}
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
	order.ReduceOnly = input.ReduceOnly
	order.CloseOnTrigger = input.CloseOnTrigger
	order.CreatedAt = types.NanoTime()
	order.UpdatedAt = order.CreatedAt

	result := pool.GetOrderResult()
	result.Order = order
	result.Trades = nil

	if input.TriggerPrice > 0 {
		order.Status = constants.ORDER_STATUS_UNTRIGGERED
		result.Status = constants.ORDER_STATUS_UNTRIGGERED
		l.getTriggerMonitor(input.Symbol).AddOrder(order)
		return result, nil
	}

	ob := l.getOrderBook(input.Symbol)
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

func (l *Linear) CancelOrder(orderID types.OrderID, userID types.UserID) error {
	order := l.orderStore.Get(orderID)
	if order == nil {
		return nil
	}
	if userID != 0 && order.UserID != userID {
		return nil
	}

	if order.Status == constants.ORDER_STATUS_NEW {
		leverage := position.GetLeverage(l.state, order.UserID, order.Symbol)
		margin := balance.CalculateMargin(int64(order.Quantity-order.Filled), int64(order.Price), leverage)
		balance.Move(l.state, order.UserID, "USDT", types.BUCKET_LOCKED, types.BUCKET_AVAILABLE, margin)
		balance.Move(l.state, order.UserID, "USDT", types.BUCKET_MARGIN, types.BUCKET_AVAILABLE, margin)

		ob := l.getOrderBook(order.Symbol)
		ob.RemoveOrder(order)
	} else if order.Status == constants.ORDER_STATUS_UNTRIGGERED {
		l.getTriggerMonitor(order.Symbol).RemoveOrder(orderID)
	}

	order.Status = constants.ORDER_STATUS_CANCELED
	l.orderStore.Remove(orderID)
	pool.PutOrder(order)
	return nil
}

func (l *Linear) GetOrderBook(symbol string, limit int) (bids, asks []int64) {
	ob := l.orderBooks[symbol]
	if ob == nil {
		return nil, nil
	}
	bids = ob.GetDepth(constants.ORDER_SIDE_BUY, limit)
	asks = ob.GetDepth(constants.ORDER_SIDE_SELL, limit)
	return bids, asks
}

func (l *Linear) OnTrigger(order *types.Order) {
	order.Status = constants.ORDER_STATUS_TRIGGERED

	if order.CloseOnTrigger {
		pos := position.GetPosition(l.state, order.UserID, order.Symbol)
		if pos != nil && pos.Size > 0 {
		}
		l.orderStore.Remove(order.ID)
		pool.PutOrder(order)
		return
	}

	_, _ = l.PlaceOrder(&types.OrderInput{
		UserID:     order.UserID,
		Symbol:     order.Symbol,
		Side:       order.Side,
		Type:       constants.ORDER_TYPE_LIMIT,
		TIF:        constants.TIF_GTC,
		Quantity:   order.Quantity - order.Filled,
		Price:      order.Price,
		ReduceOnly: order.ReduceOnly,
	})
	l.orderStore.Remove(order.ID)
	pool.PutOrder(order)
}

func (l *Linear) AdjustReduceOnlyOrders(userID types.UserID, symbol string) {
	position.AdjustReduceOnlyOrders(l.orderStore, l.state, userID, symbol)
}
