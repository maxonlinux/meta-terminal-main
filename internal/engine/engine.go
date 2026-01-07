package engine

import (
	"errors"

	"github.com/anomalyco/meta-terminal-go/internal/balance"
	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/memory"
	"github.com/anomalyco/meta-terminal-go/internal/orderbook"
	"github.com/anomalyco/meta-terminal-go/internal/position"
	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/internal/trade"
	"github.com/anomalyco/meta-terminal-go/internal/trigger"
	"github.com/anomalyco/meta-terminal-go/internal/types"
	"github.com/anomalyco/meta-terminal-go/internal/wal"
)

var (
	ErrInsufficientBalance = errors.New("insufficient balance for new margin requirement")
	ErrPositionLiquidated  = errors.New("position would be liquidated with new leverage")
	ErrInvalidLeverage     = errors.New("leverage must be between 2 and 100")
)

type Engine struct {
	wal        *wal.WAL
	state      *state.State
	monitor    *trigger.Monitor
	orderStore *memory.OrderStore
	ob         *orderbook.OrderBook
}

func New(w *wal.WAL, s *state.State) *Engine {
	orderStore := memory.NewOrderStore()
	return &Engine{
		wal:        w,
		state:      s,
		monitor:    trigger.NewMonitor(s, orderStore),
		orderStore: orderStore,
		ob:         orderbook.New(),
	}
}

func (e *Engine) PlaceOrder(input *types.OrderInput) (*types.OrderResult, error) {
	ss := e.state.GetSymbolState(input.Symbol)
	category := ss.Category
	if category == 0 && input.Category > 0 {
		category = input.Category
		ss.Category = category
	}

	if input.ReduceOnly {
		if category == constants.CATEGORY_SPOT {
			return nil, nil
		}
		if err := position.ReduceOnlyValidate(e.state, input.UserID, input.Symbol, input.Quantity, input.Side); err != nil {
			return nil, err
		}
	}

	if input.CloseOnTrigger && category == constants.CATEGORY_SPOT {
		input.CloseOnTrigger = false
	}

	orderID := e.state.NextOrderID
	e.state.NextOrderID++

	order := e.newOrder(orderID, input)
	e.orderStore.Add(order)

	result := memory.GetOrderResultPool().Get()
	result.Order = order
	result.Trades = nil

	if input.TriggerPrice > 0 {
		order.Status = constants.ORDER_STATUS_UNTRIGGERED
		result.Status = constants.ORDER_STATUS_UNTRIGGERED
		e.monitor.AddOrder(order)
		e.logOrderOp(wal.OP_PLACE_ORDER, input.UserID, input.Symbol, orderID)
		return result, nil
	}

	switch input.Type {
	case constants.ORDER_TYPE_MARKET:
		e.handleMarketOrder(order, input, result)
	case constants.ORDER_TYPE_LIMIT:
		e.handleLimitOrder(order, input, result)
	}

	e.logOrderOp(wal.OP_PLACE_ORDER, input.UserID, input.Symbol, orderID)
	return result, nil
}

func (e *Engine) newOrder(id types.OrderID, input *types.OrderInput) *types.Order {
	o := memory.GetOrderPool().Get()
	o.ID = id
	o.UserID = input.UserID
	o.Symbol = input.Symbol
	o.Side = input.Side
	o.Type = input.Type
	o.TIF = input.TIF
	o.Status = constants.ORDER_STATUS_NEW
	o.Price = input.Price
	o.Quantity = input.Quantity
	o.Filled = 0
	o.TriggerPrice = input.TriggerPrice
	o.StopOrderType = input.StopOrderType
	o.ReduceOnly = input.ReduceOnly
	o.CloseOnTrigger = input.CloseOnTrigger
	o.CreatedAt = types.NanoTime()
	o.UpdatedAt = o.CreatedAt
	return o
}

func (e *Engine) handleMarketOrder(order *types.Order, input *types.OrderInput, result *types.OrderResult) {
	if input.TIF == constants.TIF_FOK {
		e.executeFOKOrder(order, result)
		return
	}

	filled := e.matchOrder(order)
	order.Filled = filled

	switch {
	case filled == 0:
		order.Status, result.Status = constants.ORDER_STATUS_CANCELED, constants.ORDER_STATUS_CANCELED
	case filled < order.Quantity:
		order.Status, result.Status = constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED, constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED
	default:
		order.Status, result.Status = constants.ORDER_STATUS_FILLED, constants.ORDER_STATUS_FILLED
	}

	result.Filled = filled
	result.Remaining = order.Quantity - filled
}

func (e *Engine) handleLimitOrder(order *types.Order, input *types.OrderInput, result *types.OrderResult) {
	ss := e.state.GetSymbolState(order.Symbol)

	switch input.TIF {
	case constants.TIF_GTC, constants.TIF_POST_ONLY:
		if input.TIF == constants.TIF_POST_ONLY && e.ob.WouldCross(ss, order) {
			order.Status, result.Status = constants.ORDER_STATUS_CANCELED, constants.ORDER_STATUS_CANCELED
			e.logOrderOp(wal.OP_CANCEL_ORDER, input.UserID, input.Symbol, order.ID)
			return
		}
		balance.LockForOrder(e.state, e.state.GetSymbolState(order.Symbol).Category, order.UserID, order, position.GetLeverage(e.state, order.UserID, order.Symbol))

		filled := e.matchOrder(order)
		if filled > 0 {
			order.Filled = filled
			order.Status = constants.ORDER_STATUS_PARTIALLY_FILLED
			result.Status = constants.ORDER_STATUS_PARTIALLY_FILLED
			result.Filled = filled
			result.Remaining = order.Quantity - filled
			if filled >= order.Quantity {
				order.Status = constants.ORDER_STATUS_FILLED
				result.Status = constants.ORDER_STATUS_FILLED
			}
		} else {
			order.Status, result.Status = constants.ORDER_STATUS_NEW, constants.ORDER_STATUS_NEW
			e.ob.AddOrder(ss, order)
		}

	case constants.TIF_IOC:
		filled := e.matchOrder(order)
		result.Filled, result.Remaining = filled, order.Quantity-filled
		switch {
		case filled == 0:
			order.Status, result.Status = constants.ORDER_STATUS_CANCELED, constants.ORDER_STATUS_CANCELED
		case filled < order.Quantity:
			order.Status, result.Status = constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED, constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED
		default:
			order.Status, result.Status = constants.ORDER_STATUS_FILLED, constants.ORDER_STATUS_FILLED
		}

	case constants.TIF_FOK:
		e.executeFOKOrder(order, result)
	}
}

func (e *Engine) executeFOKOrder(order *types.Order, result *types.OrderResult) {
	price := int64(0)
	if order.Type == constants.ORDER_TYPE_LIMIT {
		price = int64(order.Price)
	}

	ss := e.state.GetSymbolState(order.Symbol)
	available := e.ob.AvailableQuantity(ss, order.Side, price, order.Quantity)
	if available < order.Quantity {
		order.Status, result.Status = constants.ORDER_STATUS_CANCELED, constants.ORDER_STATUS_CANCELED
		result.Filled, result.Remaining = 0, order.Quantity
		return
	}

	filled := e.matchOrder(order)
	if filled < order.Quantity {
		order.Status, result.Status = constants.ORDER_STATUS_CANCELED, constants.ORDER_STATUS_CANCELED
		result.Filled, result.Remaining = 0, order.Quantity
		return
	}

	order.Status, result.Status = constants.ORDER_STATUS_FILLED, constants.ORDER_STATUS_FILLED
	result.Filled, result.Remaining = filled, 0
}

func (e *Engine) CancelOrder(orderID types.OrderID, userID types.UserID) error {
	order := e.orderStore.Get(orderID)
	if order == nil {
		return nil
	}
	if userID != 0 && order.UserID != userID {
		return nil
	}

	ss := e.state.GetSymbolState(order.Symbol)

	switch order.Status {
	case constants.ORDER_STATUS_NEW:
		e.ob.RemoveOrder(ss, order)
		balance.UnlockForOrder(e.state, e.state.GetSymbolState(order.Symbol).Category, order.UserID, order, position.GetLeverage(e.state, order.UserID, order.Symbol))
	case constants.ORDER_STATUS_UNTRIGGERED:
		e.monitor.RemoveOrder(orderID, order.Symbol)
	case constants.ORDER_STATUS_PARTIALLY_FILLED:
		balance.UnlockForOrder(e.state, e.state.GetSymbolState(order.Symbol).Category, order.UserID, order, position.GetLeverage(e.state, order.UserID, order.Symbol))
	default:
		return nil
	}

	order.Status = constants.ORDER_STATUS_CANCELED
	order.UpdatedAt = types.NanoTime()
	e.logOrderOp(wal.OP_CANCEL_ORDER, order.UserID, order.Symbol, orderID)
	e.orderStore.Remove(orderID)
	memory.GetOrderPool().Put(order)
	return nil
}

func (e *Engine) InitSymbolCategory(symbol types.SymbolID, category int8) {
	e.state.GetSymbolState(symbol).Category = category
}

func (e *Engine) AmendOrder(orderID types.OrderID, userID types.UserID, newQty types.Quantity, newPrice types.Price) error {
	order := e.orderStore.Get(orderID)
	if order == nil || order.UserID != userID || order.Status != constants.ORDER_STATUS_NEW {
		return nil
	}

	oldQty, oldPrice := order.Quantity, order.Price
	order.Quantity, order.Price, order.UpdatedAt = newQty, newPrice, types.NanoTime()

	balance.AdjustLocked(e.state, e.state.GetSymbolState(order.Symbol).Category, order.UserID, order, oldQty, oldPrice, position.GetLeverage(e.state, order.UserID, order.Symbol))
	e.logOrderOp(wal.OP_AMEND_ORDER, userID, order.Symbol, orderID)
	return nil
}

func (e *Engine) GetOrder(orderID types.OrderID) *types.Order {
	return e.orderStore.Get(orderID)
}

func (e *Engine) GetUserOrders(userID types.UserID) []*types.Order {
	orderIDs := e.orderStore.GetUserOrders(userID)
	orders := make([]*types.Order, 0, len(orderIDs))
	for _, orderID := range orderIDs {
		order := e.orderStore.Get(orderID)
		if order != nil {
			orders = append(orders, order)
		}
	}
	return orders
}

func (e *Engine) GetUserBalances(userID types.UserID) []*types.UserBalance {
	us := e.state.GetUserState(userID)
	balances := make([]*types.UserBalance, 0, len(us.Balances))
	for _, b := range us.Balances {
		balances = append(balances, b)
	}
	return balances
}

func (e *Engine) GetUserPosition(userID types.UserID, symbol types.SymbolID) *types.Position {
	return e.state.GetUserState(userID).Positions[symbol]
}

func (e *Engine) GetUserBalanceByAsset(userID types.UserID, asset string) *types.UserBalance {
	return e.state.GetUserState(userID).Balances[asset]
}

func (e *Engine) GetOrderBook(symbol types.SymbolID, limit int) (bids, asks []int64) {
	ss := e.state.GetSymbolState(symbol)
	return e.ob.GetDepth(ss, constants.ORDER_SIDE_BUY, limit), e.ob.GetDepth(ss, constants.ORDER_SIDE_SELL, limit)
}

func (e *Engine) ClosePosition(userID types.UserID, symbol types.SymbolID) error {
	pos := e.state.GetUserState(userID).Positions[symbol]
	if pos == nil || pos.Size == 0 {
		return nil
	}

	closeSide := int8(constants.ORDER_SIDE_SELL)
	if pos.Side == constants.ORDER_SIDE_SELL {
		closeSide = int8(constants.ORDER_SIDE_BUY)
	}

	_, _ = e.PlaceOrder(&types.OrderInput{
		UserID:         userID,
		Symbol:         symbol,
		Side:           closeSide,
		Type:           constants.ORDER_TYPE_MARKET,
		TIF:            constants.TIF_IOC,
		Quantity:       pos.Size,
		CloseOnTrigger: true,
		ReduceOnly:     true,
	})
	return nil
}

func (e *Engine) EditLeverage(userID types.UserID, symbol types.SymbolID, leverage int8) error {
	if leverage < 1 || leverage > 100 {
		return ErrInvalidLeverage
	}

	pos := e.state.GetUserState(userID).Positions[symbol]

	if pos == nil {
		e.state.GetUserState(userID).Positions[symbol] = &types.Position{
			UserID: userID, Symbol: symbol, Size: 0, Side: -1, Leverage: leverage,
		}
		return nil
	}

	if pos.Size == 0 {
		pos.Leverage = leverage
		return nil
	}

	oldMargin := pos.InitialMargin
	newMargin := position.CalculateMargin(pos.Size, pos.EntryPrice, leverage)
	marginDiff := newMargin - oldMargin

	bal := balance.GetOrCreate(e.state, userID, "USDT")
	if marginDiff > 0 {
		if bal.Available < marginDiff {
			return ErrInsufficientBalance
		}
	}

	currentPrice := e.getCurrentPrice(symbol)
	if currentPrice > 0 {
		upnl := (int64(currentPrice) - int64(pos.EntryPrice)) * int64(pos.Size)
		buffer := newMargin - newMargin/10
		if upnl < -buffer || upnl > buffer {
			return ErrPositionLiquidated
		}
	}

	pos.Leverage = leverage
	position.CalculatePositionRisk(pos)
	bal.Margin = pos.InitialMargin

	if marginDiff > 0 {
		bal.Available -= marginDiff
	} else if marginDiff < 0 {
		bal.Available += -marginDiff
	}

	return nil
}

func (e *Engine) EditTPSL(userID types.UserID, symbol types.SymbolID, tpOrderID, slOrderID int64, tpPrice, slPrice types.Price) error {
	pos := e.state.GetUserState(userID).Positions[symbol]
	if pos == nil || pos.Size == 0 {
		return nil
	}

	tpSide := int8(constants.ORDER_SIDE_SELL)
	if pos.Side == constants.ORDER_SIDE_SELL {
		tpSide = int8(constants.ORDER_SIDE_BUY)
	}

	// tp order
	_, _ = e.PlaceOrder(&types.OrderInput{
		UserID:         userID,
		Symbol:         symbol,
		Side:           tpSide,
		Type:           constants.ORDER_TYPE_LIMIT,
		TIF:            constants.TIF_GTC,
		Quantity:       pos.Size,
		Price:          tpPrice,
		TriggerPrice:   tpPrice,
		StopOrderType:  constants.STOP_ORDER_TYPE_TP,
		CloseOnTrigger: true,
		ReduceOnly:     true,
	})

	// sl order
	_, _ = e.PlaceOrder(&types.OrderInput{
		UserID:         userID,
		Symbol:         symbol,
		Side:           tpSide,
		Type:           constants.ORDER_TYPE_LIMIT,
		TIF:            constants.TIF_GTC,
		Quantity:       pos.Size,
		Price:          slPrice,
		TriggerPrice:   slPrice,
		StopOrderType:  constants.STOP_ORDER_TYPE_SL,
		CloseOnTrigger: true,
		ReduceOnly:     true,
	})
	return nil
}

func (e *Engine) OnTrigger(order *types.Order) {
	order.Status = constants.ORDER_STATUS_TRIGGERED

	if order.CloseOnTrigger {
		pos := e.state.GetUserState(order.UserID).Positions[order.Symbol]
		if pos != nil && pos.Size != 0 {
			_ = e.ClosePosition(order.UserID, order.Symbol)
		}
		memory.GetOrderPool().Put(order)
		return
	}

	_, _ = e.PlaceOrder(&types.OrderInput{
		UserID:        order.UserID,
		Symbol:        order.Symbol,
		Side:          order.Side,
		Type:          constants.ORDER_TYPE_LIMIT,
		TIF:           constants.TIF_GTC,
		Price:         order.Price,
		Quantity:      order.Quantity - order.Filled,
		StopOrderType: constants.STOP_ORDER_TYPE_NORMAL,
		ReduceOnly:    order.ReduceOnly,
	})
	memory.GetOrderPool().Put(order)
}

func (e *Engine) getCurrentPrice(symbol types.SymbolID) types.Price {
	ss := e.state.GetSymbolState(symbol)
	bestBid := e.ob.GetBestBid(ss)
	if bestBid > 0 {
		return bestBid
	}
	return e.ob.GetBestAsk(ss)
}

func (e *Engine) matchOrder(order *types.Order) types.Quantity {
	ss := e.state.GetSymbolState(order.Symbol)
	var filled types.Quantity
	if order.Side == constants.ORDER_SIDE_BUY {
		current := ss.BestAsk
		for current != nil && order.Quantity > filled {
			filled += e.matchAtLevel(order, current, ss.AskIndex)
			current = current.NextAsk
		}
	} else {
		current := ss.BestBid
		for current != nil && order.Quantity > filled {
			filled += e.matchAtLevel(order, current, ss.BidIndex)
			current = current.NextBid
		}
	}
	return filled
}

func (e *Engine) matchAtLevel(order *types.Order, level *state.PriceLevel, levels map[types.Price]*state.PriceLevel) types.Quantity {
	if level == nil || level.Orders.Len() == 0 {
		return 0
	}

	var filled types.Quantity
	tradePrice := level.Price
	category := e.state.GetSymbolState(order.Symbol).Category
	needsSideCheck := category == constants.CATEGORY_LINEAR

	for order.Quantity > filled {
		currentOID := level.Orders.Peek()
		if currentOID == 0 {
			break
		}

		maker := e.orderStore.Get(currentOID)
		if maker == nil {
			level.Orders.Pop()
			continue
		}
		if maker.Status == constants.ORDER_STATUS_FILLED || maker.Status == constants.ORDER_STATUS_CANCELED {
			level.Orders.Pop()
			level.Quantity -= maker.Quantity
			continue
		}

		tradeQty := min(order.Quantity-filled, maker.Quantity-maker.Filled)

		if order.Price > 0 {
			if order.Side == constants.ORDER_SIDE_BUY && order.Price < types.Price(level.Price) {
				break
			}
			if order.Side == constants.ORDER_SIDE_SELL && order.Price > types.Price(level.Price) {
				break
			}
		}

		if needsSideCheck {
			lev := position.GetLeverage(e.state, order.UserID, order.Symbol)
			trade.ExecuteLinearTrade(e.state, order, maker, tradePrice, tradeQty, lev)
			e.logFill(order, maker, tradePrice, tradeQty)
			e.logPositionUpdate(order.UserID, order.Symbol)
			e.logPositionUpdate(maker.UserID, maker.Symbol)
		} else {
			trade.ExecuteSpotTrade(e.state, order, maker, tradePrice, tradeQty)
			e.logFill(order, maker, tradePrice, tradeQty)
		}

		maker.Filled += tradeQty
		filled += tradeQty
		order.Filled += tradeQty
		level.Quantity -= tradeQty

		if maker.Filled >= maker.Quantity {
			maker.Status = constants.ORDER_STATUS_FILLED
			level.Orders.Pop()
		}

		if order.Filled >= order.Quantity {
			order.Status = constants.ORDER_STATUS_FILLED
			break
		}
	}

	if level.Orders.Len() == 0 || level.Quantity == 0 {
		delete(levels, level.Price)
		if order.Side == constants.ORDER_SIDE_BUY {
			e.ob.MarkLevelEmpty(e.state.GetSymbolState(order.Symbol), false, level.Price)
		} else {
			e.ob.MarkLevelEmpty(e.state.GetSymbolState(order.Symbol), true, level.Price)
		}
	}

	return filled
}

func (e *Engine) logOrderOp(opType wal.OperationType, userID types.UserID, symbol types.SymbolID, orderID types.OrderID) {
	if e.wal == nil {
		return
	}
	_ = e.wal.Append(&wal.Operation{
		Type:      opType,
		Timestamp: int64(types.NanoTime()),
		UserID:    int64(userID),
		Symbol:    int32(symbol),
		OrderID:   int64(orderID),
	})
}

func (e *Engine) logFill(taker, maker *types.Order, price types.Price, qty types.Quantity) {
	if e.wal == nil {
		return
	}
	_ = e.wal.AppendFill(&wal.FillOp{
		TakerOrderID: int64(taker.ID),
		MakerOrderID: int64(maker.ID),
		Price:        int64(price),
		Quantity:     int64(qty),
		Symbol:       int32(taker.Symbol),
		TakerUserID:  int64(taker.UserID),
		MakerUserID:  int64(maker.UserID),
	})
}

func (e *Engine) logPositionUpdate(userID types.UserID, symbol types.SymbolID) {
	if e.wal == nil {
		return
	}
	us := e.state.GetUserState(userID)
	pos := us.Positions[symbol]
	if pos == nil {
		return
	}
	_ = e.wal.AppendPositionUpdate(&wal.PositionUpdateOp{
		UserID:     int64(userID),
		Symbol:     int32(symbol),
		Size:       int64(pos.Size),
		Side:       pos.Side,
		EntryPrice: int64(pos.EntryPrice),
		Leverage:   pos.Leverage,
	})
}
