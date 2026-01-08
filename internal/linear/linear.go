package linear

import (
	"errors"

	"github.com/anomalyco/meta-terminal-go/internal/balance"
	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/orderbook"
	"github.com/anomalyco/meta-terminal-go/internal/pool"
	"github.com/anomalyco/meta-terminal-go/internal/position"
	"github.com/anomalyco/meta-terminal-go/internal/price"
	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/internal/trigger"
	"github.com/anomalyco/meta-terminal-go/types"
)

var (
	ErrInsufficientBalance = errors.New("insufficient balance for leverage change")
	ErrWouldLiquidate      = errors.New("leverage change would trigger liquidation")
)

type Linear struct {
	state      *state.EngineState
	orderStore *state.OrderStore
	orderBooks map[string]*orderbook.OrderBook
	triggerMon map[string]*trigger.Monitor
	priceFeed  *price.Feed
}

func New(s *state.EngineState, orderStore *state.OrderStore) *Linear {
	return &Linear{
		state:      s,
		orderStore: orderStore,
		orderBooks: make(map[string]*orderbook.OrderBook),
		triggerMon: make(map[string]*trigger.Monitor),
		priceFeed:  price.NewFeed(s),
	}
}

func (l *Linear) Reset() {
	l.orderBooks = make(map[string]*orderbook.OrderBook)
	l.triggerMon = make(map[string]*trigger.Monitor)
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

func (l *Linear) OnPriceTick(symbol string, markPrice types.Price) {
	l.priceFeed.UpdatePrice(symbol, markPrice)
}

func (l *Linear) PlaceOrder(input *types.OrderInput) (*types.OrderResult, error) {
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

	if input.ReduceOnly {
		if err := position.ReduceOnlyValidate(l.state, input.UserID, input.Symbol, input.Quantity, input.Side); err != nil {
			pool.PutOrder(order)
			return nil, err
		}
	}

	lev := position.GetLeverage(l.state, input.UserID, input.Symbol)
	reserved := balance.CalculateMargin(int64(order.Quantity), int64(order.Price), lev)

	if err := balance.Deduct(l.state, input.UserID, "USDT", types.BUCKET_AVAILABLE, reserved); err != nil {
		pool.PutOrder(order)
		return nil, err
	}
	if err := balance.Add(l.state, input.UserID, "USDT", types.BUCKET_LOCKED, reserved); err != nil {
		balance.Add(l.state, input.UserID, "USDT", types.BUCKET_AVAILABLE, reserved)
		pool.PutOrder(order)
		return nil, err
	}

	trades, err := ob.AddOrder(order)
	if err != nil {
		l.refundUnfilled(order, reserved)
		pool.PutOrder(order)
		return nil, err
	}
	result.Trades = trades

	l.executeLinearTrades(trades, order, reserved)

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
			l.refundUnfilled(order, reserved)
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
			l.refundUnfilled(order, reserved)
		}
	}

	result.Filled = order.Filled
	result.Remaining = order.Quantity - order.Filled

	return result, nil
}

func (l *Linear) refundUnfilled(order *types.Order, reserved int64) {
	unfilledQty := order.Quantity - order.Filled
	orderReserved := int64(unfilledQty) * int64(order.Price)

	balance.Move(l.state, order.UserID, "USDT", types.BUCKET_LOCKED, types.BUCKET_AVAILABLE, orderReserved)
}

func (l *Linear) executeLinearTrades(trades []*types.Trade, taker *types.Order, reserved int64) {
	lev := position.GetLeverage(l.state, taker.UserID, taker.Symbol)

	for _, trade := range trades {
		maker := l.orderStore.Get(trade.MakerOrderID)
		if maker == nil {
			continue
		}

		trade.TakerID = taker.UserID
		trade.MakerID = maker.UserID
		trade.Symbol = taker.Symbol
		trade.ExecutedAt = types.NanoTime()

		tradeQty := int64(trade.Quantity)
		tradePrice := int64(trade.Price)

		tradeMargin := balance.CalculateMargin(tradeQty, tradePrice, lev)

		if taker.Side == constants.ORDER_SIDE_BUY {
			balance.Move(l.state, taker.UserID, "USDT", types.BUCKET_LOCKED, types.BUCKET_AVAILABLE, int64(maker.Price)*tradeQty)
			balance.Deduct(l.state, taker.UserID, "USDT", types.BUCKET_AVAILABLE, tradeMargin)
			balance.Add(l.state, taker.UserID, "USDT", types.BUCKET_MARGIN, tradeMargin)

			position.UpdatePosition(l.state, taker.UserID, taker.Symbol, trade.Quantity, trade.Price, taker.Side, lev)
		} else {
			balance.Move(l.state, taker.UserID, "USDT", types.BUCKET_LOCKED, types.BUCKET_AVAILABLE, int64(maker.Price)*tradeQty)
			balance.Deduct(l.state, taker.UserID, "USDT", types.BUCKET_AVAILABLE, tradeMargin)
			balance.Add(l.state, taker.UserID, "USDT", types.BUCKET_MARGIN, tradeMargin)

			position.UpdatePosition(l.state, taker.UserID, taker.Symbol, trade.Quantity, trade.Price, taker.Side, lev)
		}

		position.UpdatePosition(l.state, maker.UserID, maker.Symbol, trade.Quantity, trade.Price, maker.Side, lev)
	}

	if taker.Filled > 0 {
		filledMargin := balance.CalculateMargin(int64(taker.Filled), int64(taker.Price), lev)
		totalReleased := reserved - filledMargin
		if totalReleased > 0 {
			balance.Move(l.state, taker.UserID, "USDT", types.BUCKET_LOCKED, types.BUCKET_AVAILABLE, totalReleased)
		}
	}
}

func (l *Linear) SetLeverage(userID types.UserID, symbol string, leverage int8) error {
	pos := position.GetPosition(l.state, userID, symbol)

	if pos == nil || pos.Size == 0 {
		us := l.state.GetUserState(userID)
		if us.Positions[symbol] == nil {
			us.Positions[symbol] = &types.Position{
				UserID:   userID,
				Symbol:   symbol,
				Size:     0,
				Side:     types.SIDE_NONE,
				Leverage: leverage,
			}
		} else {
			us.Positions[symbol].Leverage = leverage
		}
		return nil
	}

	oldLeverage := pos.Leverage
	oldLiquidationPrice := pos.LiquidationPrice

	pos.Leverage = leverage
	position.CalculatePositionRisk(pos)

	currentPrice := l.priceFeed.GetPrice(symbol)
	if currentPrice == 0 {
		currentPrice = pos.EntryPrice
	}

	if pos.Side == constants.ORDER_SIDE_BUY && pos.LiquidationPrice >= currentPrice {
		pos.Leverage = oldLeverage
		position.CalculatePositionRisk(pos)
		return ErrWouldLiquidate
	}

	if pos.Side == constants.ORDER_SIDE_SELL && pos.LiquidationPrice <= currentPrice {
		pos.Leverage = oldLeverage
		position.CalculatePositionRisk(pos)
		return ErrWouldLiquidate
	}

	requiredMargin := balance.CalculateMargin(int64(pos.Size), int64(currentPrice), leverage)
	currentMargin := balance.Get(l.state, userID, "USDT", types.BUCKET_MARGIN)

	if requiredMargin > currentMargin {
		additional := requiredMargin - currentMargin
		available := balance.Get(l.state, userID, "USDT", types.BUCKET_AVAILABLE)
		if available < additional {
			pos.Leverage = oldLeverage
			position.CalculatePositionRisk(pos)
			return ErrInsufficientBalance
		}
		balance.Deduct(l.state, userID, "USDT", types.BUCKET_AVAILABLE, additional)
		balance.Add(l.state, userID, "USDT", types.BUCKET_MARGIN, additional)
	} else if requiredMargin < currentMargin {
		release := currentMargin - requiredMargin
		balance.Move(l.state, userID, "USDT", types.BUCKET_MARGIN, types.BUCKET_AVAILABLE, release)
	}

	_ = oldLiquidationPrice

	return nil
}

func (l *Linear) CancelOrder(orderID types.OrderID, userID types.UserID) error {
	order := l.orderStore.Get(orderID)
	if order == nil {
		return nil
	}
	if userID != 0 && order.UserID != userID {
		return nil
	}

	if order.Status == constants.ORDER_STATUS_NEW || order.Status == constants.ORDER_STATUS_PARTIALLY_FILLED {
		l.refundUnfilled(order, 0)

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
