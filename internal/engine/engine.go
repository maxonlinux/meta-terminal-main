package engine

import (
	"errors"
	"math"
	"time"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/internal/trigger"
	"github.com/anomalyco/meta-terminal-go/internal/types"
	"github.com/anomalyco/meta-terminal-go/internal/wal"
)

var (
	ErrInsufficientBalance = errors.New("insufficient balance for new margin requirement")
	ErrPositionLiquidated  = errors.New("position would be liquidated with new leverage")
	ErrInvalidLeverage     = errors.New("leverage must be between 1 and 100")
)

type Engine struct {
	wal     *wal.WAL
	state   *state.State
	monitor *trigger.Monitor
}

func New(w *wal.WAL, s *state.State) *Engine {
	return &Engine{wal: w, state: s, monitor: trigger.NewMonitor(s)}
}

func (e *Engine) PlaceOrder(input *types.OrderInput) (*types.OrderResult, error) {
	now := time.Now()
	category := e.getSymbolCategory(input.Symbol)

	if input.ReduceOnly {
		if category == constants.CATEGORY_SPOT {
			return nil, nil
		}
		if err := e.validateReduceOnly(input); err != nil {
			return nil, err
		}
	}

	orderID := e.state.NextOrderID
	e.state.NextOrderID++

	order := &types.Order{
		ID:             orderID,
		UserID:         input.UserID,
		Symbol:         input.Symbol,
		Side:           input.Side,
		Type:           input.Type,
		TIF:            input.TIF,
		Status:         constants.ORDER_STATUS_NEW,
		Price:          input.Price,
		Quantity:       input.Quantity,
		Filled:         0,
		TriggerPrice:   input.TriggerPrice,
		StopOrderType:  input.StopOrderType,
		ReduceOnly:     input.ReduceOnly,
		CloseOnTrigger: input.CloseOnTrigger,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	e.state.AddOrder(order)
	result := &types.OrderResult{Order: order, Trades: make([]*types.Trade, 0)}

	if input.ReduceOnly {
		return e.executeReduceOnly(order, input, result, now)
	}

	if input.TriggerPrice > 0 {
		order.Status = constants.ORDER_STATUS_UNTRIGGERED
		result.Status = constants.ORDER_STATUS_UNTRIGGERED
		e.monitor.AddOrder(order)
		e.logOrderOp(wal.OP_PLACE_ORDER, now, input.UserID, input.Symbol, orderID)
		return result, nil
	}

	switch input.Type {
	case constants.ORDER_TYPE_MARKET:
		e.handleMarketOrder(order, input, result, now)
	case constants.ORDER_TYPE_LIMIT:
		e.handleLimitOrder(order, input, result, now)
	}

	e.logOrderOp(wal.OP_PLACE_ORDER, now, input.UserID, input.Symbol, orderID)
	return result, nil
}

func (e *Engine) handleMarketOrder(order *types.Order, input *types.OrderInput, result *types.OrderResult, now time.Time) {
	if input.TIF == constants.TIF_FOK {
		e.executeFOKOrder(order, result)
		return
	}

	filled := e.matchOrder(order)
	order.Filled = filled

	switch {
	case filled == 0:
		e.setStatus(order, result, constants.ORDER_STATUS_CANCELED)
	case filled < order.Quantity:
		e.setStatus(order, result, constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED)
	default:
		e.setStatus(order, result, constants.ORDER_STATUS_FILLED)
	}

	result.Filled = filled
	result.Remaining = order.Quantity - filled
}

func (e *Engine) handleLimitOrder(order *types.Order, input *types.OrderInput, result *types.OrderResult, now time.Time) {
	switch input.TIF {
	case constants.TIF_GTC, constants.TIF_POST_ONLY:
		if input.TIF == constants.TIF_POST_ONLY && e.checkWouldCross(input.Symbol, input.Side, input.Price) {
			e.setStatus(order, result, constants.ORDER_STATUS_CANCELED)
			e.logOrderOp(wal.OP_CANCEL_ORDER, now, input.UserID, input.Symbol, order.ID)
			return
		}
		e.lockBalanceForOrder(order)
		e.setStatus(order, result, constants.ORDER_STATUS_NEW)
		e.addToOrderBook(order)

	case constants.TIF_IOC:
		filled := e.matchOrder(order)
		result.Filled, result.Remaining = filled, order.Quantity-filled
		switch {
		case filled == 0:
			e.setStatus(order, result, constants.ORDER_STATUS_CANCELED)
		case filled < order.Quantity:
			e.setStatus(order, result, constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED)
		default:
			e.setStatus(order, result, constants.ORDER_STATUS_FILLED)
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

	available := e.getAvailableQuantity(order.Symbol, order.Side, price)
	if available < order.Quantity {
		result.Filled, result.Remaining = 0, order.Quantity
		e.setStatus(order, result, constants.ORDER_STATUS_CANCELED)
		return
	}

	filled := e.matchOrder(order)
	if filled < order.Quantity {
		result.Filled, result.Remaining = 0, order.Quantity
		e.setStatus(order, result, constants.ORDER_STATUS_CANCELED)
		return
	}

	result.Filled, result.Remaining = filled, 0
	e.setStatus(order, result, constants.ORDER_STATUS_FILLED)
}

func (e *Engine) CancelOrder(orderID types.OrderID, userID types.UserID) error {
	order := e.getOrderByID(orderID)
	if order == nil || (userID != 0 && order.UserID != userID) {
		return nil
	}

	switch order.Status {
	case constants.ORDER_STATUS_NEW:
		e.removeFromOrderBook(order)
		e.unlockBalanceForOrder(order)
	case constants.ORDER_STATUS_UNTRIGGERED:
		e.monitor.RemoveOrder(orderID, order.Symbol)
	case constants.ORDER_STATUS_PARTIALLY_FILLED:
		e.unlockBalanceForOrder(order)
	default:
		return nil
	}

	order.Status = constants.ORDER_STATUS_CANCELED
	order.UpdatedAt = time.Now()
	e.logOrderOp(wal.OP_CANCEL_ORDER, time.Now(), order.UserID, order.Symbol, orderID)
	return nil
}

func (e *Engine) AmendOrder(orderID types.OrderID, userID types.UserID, newQty types.Quantity, newPrice types.Price) error {
	order := e.getOrderByID(orderID)
	if order == nil || order.UserID != userID || order.Status != constants.ORDER_STATUS_NEW {
		return nil
	}

	oldQty, oldPrice := order.Quantity, order.Price
	order.Quantity, order.Price, order.UpdatedAt = newQty, newPrice, time.Now()

	if oldPrice != newPrice {
		ss := e.state.GetSymbolState(order.Symbol)
		delete(ss.OrderMap, orderID)
		ss.OrderMap[orderID] = order
	}

	e.adjustLockedBalance(order, oldQty, oldPrice)
	e.logOrderOp(wal.OP_AMEND_ORDER, time.Now(), userID, order.Symbol, orderID)
	return nil
}

func (e *Engine) GetOrder(orderID types.OrderID) *types.Order {
	return e.getOrderByID(orderID)
}

func (e *Engine) InitSymbolCategory(symbol types.SymbolID, category int8) {
	e.state.GetSymbolState(symbol).Category = category
}

func (e *Engine) GetUserOrders(userID types.UserID) []*types.Order {
	userOrders := e.state.UsersOrders[userID]
	if userOrders == nil {
		return nil
	}
	orders := make([]*types.Order, 0, len(userOrders))
	for _, o := range userOrders {
		orders = append(orders, o)
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

func (e *Engine) GetOrderBook(symbol types.SymbolID, limit int) (bids, asks [][]int64) {
	ss := e.state.GetSymbolState(symbol)
	for pl, arr := ss.Bids, &bids; len(*arr) < limit && pl != nil; pl = pl.NextPriceLevel {
		*arr = append(*arr, []int64{int64(pl.Price), int64(pl.Quantity)})
	}
	for pl, arr := ss.Asks, &asks; len(*arr) < limit && pl != nil; pl = pl.NextPriceLevel {
		*arr = append(*arr, []int64{int64(pl.Price), int64(pl.Quantity)})
	}
	return
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

	oldLev := int(pos.Leverage)
	if oldLev == 0 {
		oldLev = 2
	}

	value := abs64(int64(pos.Size) * int64(pos.EntryPrice))
	oldMargin, newMargin := value/int64(oldLev), value/int64(leverage)
	marginDiff := newMargin - oldMargin

	bal := e.getOrCreateBalance(userID, "USDT")
	if marginDiff > 0 {
		if bal.Available < marginDiff {
			return ErrInsufficientBalance
		}
		bal.Available -= marginDiff
	} else if marginDiff < 0 {
		bal.Available += -marginDiff
	}
	bal.Margin = newMargin

	liqPrice := e.calculateLiquidationPrice(pos, leverage)
	currentPrice := e.getCurrentPrice(symbol)
	if currentPrice > 0 && liqPrice > 0 && ((pos.Side == constants.ORDER_SIDE_BUY && currentPrice <= liqPrice) ||
		(pos.Side == constants.ORDER_SIDE_SELL && currentPrice >= liqPrice)) {
		return ErrPositionLiquidated
	}

	pos.Leverage = leverage
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

	createStopOrder := func(orderID int64, price types.Price, stopType int8) {
		if orderID != 0 || price == 0 {
			return
		}
		e.state.NextOrderID++
		order := &types.Order{
			ID:             e.state.NextOrderID,
			UserID:         userID,
			Symbol:         symbol,
			Side:           tpSide,
			Type:           constants.ORDER_TYPE_LIMIT,
			TIF:            constants.TIF_GTC,
			Price:          price,
			Quantity:       pos.Size,
			Status:         constants.ORDER_STATUS_UNTRIGGERED,
			TriggerPrice:   price,
			StopOrderType:  stopType,
			CloseOnTrigger: true,
			ReduceOnly:     true,
		}
		e.monitor.AddOrder(order)
	}

	createStopOrder(tpOrderID, tpPrice, constants.STOP_ORDER_TYPE_TP)
	createStopOrder(slOrderID, slPrice, constants.STOP_ORDER_TYPE_SL)
	return nil
}

func (e *Engine) OnTrigger(order *types.Order) {
	order.Status = constants.ORDER_STATUS_TRIGGERED

	if order.CloseOnTrigger {
		pos := e.state.GetUserState(order.UserID).Positions[order.Symbol]
		if pos != nil && pos.Size != 0 {
			_ = e.ClosePosition(order.UserID, order.Symbol)
		}
		return
	}

	e.state.NextOrderID++
	newOrder := &types.Order{
		ID:             e.state.NextOrderID,
		UserID:         order.UserID,
		Symbol:         order.Symbol,
		Side:           order.Side,
		Type:           constants.ORDER_TYPE_LIMIT,
		TIF:            constants.TIF_GTC,
		Price:          order.Price,
		Quantity:       order.Quantity - order.Filled,
		Status:         constants.ORDER_STATUS_NEW,
		TriggerPrice:   0,
		StopOrderType:  constants.STOP_ORDER_TYPE_NORMAL,
		ReduceOnly:     order.ReduceOnly,
		CloseOnTrigger: false,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	e.state.AddOrder(newOrder)
	e.lockBalanceForOrder(newOrder)
	e.addToOrderBook(newOrder)
}

func (e *Engine) getOrderByID(orderID types.OrderID) *types.Order {
	return e.state.OrderByID[orderID]
}

func (e *Engine) getSymbolCategory(symbol types.SymbolID) int8 {
	return e.state.GetSymbolState(symbol).Category
}

func (e *Engine) getCurrentPrice(symbol types.SymbolID) types.Price {
	ss := e.state.GetSymbolState(symbol)
	if ss.Bids != nil {
		return ss.Bids.Price
	}
	if ss.Asks != nil {
		return ss.Asks.Price
	}
	return 0
}

func (e *Engine) calculateLiquidationPrice(pos *types.Position, leverage int8) types.Price {
	if pos.Size == 0 || leverage == 0 {
		return 0
	}
	ratio := float64(leverage) / 100.0
	distance := float64(pos.EntryPrice) * ratio * 10
	if pos.Side == constants.ORDER_SIDE_BUY {
		return types.Price(float64(pos.EntryPrice) - distance)
	}
	return types.Price(float64(pos.EntryPrice) + distance)
}

func (e *Engine) validateReduceOnly(input *types.OrderInput) error {
	pos := e.state.GetUserState(input.UserID).Positions[input.Symbol]
	if pos == nil || pos.Size == 0 {
		return errors.New("reduceOnly order requires an existing position")
	}
	isClosing := (input.Side == constants.ORDER_SIDE_SELL && pos.Side == constants.ORDER_SIDE_BUY) ||
		(input.Side == constants.ORDER_SIDE_BUY && pos.Side == constants.ORDER_SIDE_SELL)
	if !isClosing {
		return errors.New("reduceOnly order must close existing position")
	}
	if input.Quantity > pos.Size {
		return errors.New("reduceOnly quantity exceeds position size")
	}
	return nil
}

func (e *Engine) addToOrderBook(order *types.Order) {
	ss := e.state.GetSymbolState(order.Symbol)
	var level **state.PriceLevel
	if order.Side == constants.ORDER_SIDE_BUY {
		level = &ss.Bids
	} else {
		level = &ss.Asks
	}

	for *level != nil && (*level).Price < order.Price {
		level = &(*level).NextPriceLevel
	}

	if *level == nil || (*level).Price != order.Price {
		newLevel := &state.PriceLevel{
			Price:    order.Price,
			Quantity: order.Quantity,
			Orders:   make(map[types.OrderID]*types.Order),
		}
		newLevel.NextPriceLevel = *level
		*level = newLevel
	} else {
		(*level).Quantity += order.Quantity
	}
	(*level).Orders[order.ID] = order
}

func (e *Engine) removeFromOrderBook(order *types.Order) {
	ss := e.state.GetSymbolState(order.Symbol)
	level := ss.Bids
	if order.Side == constants.ORDER_SIDE_SELL {
		level = ss.Asks
	}

	for level != nil && level.Price != order.Price {
		level = level.NextPriceLevel
	}

	if level != nil {
		delete(level.Orders, order.ID)
		remaining := order.Quantity - order.Filled
		level.Quantity -= remaining
		if level.Quantity <= 0 {
			if order.Side == constants.ORDER_SIDE_BUY {
				ss.Bids = level.NextPriceLevel
			} else {
				ss.Asks = level.NextPriceLevel
			}
		}
	}
}

// ONE MORE METHOD TO CHECK IF IT CROSSES!!!!
func (e *Engine) checkWouldCross(symbol types.SymbolID, side int8, price types.Price) bool {
	ss := e.state.GetSymbolState(symbol)
	if side == constants.ORDER_SIDE_BUY && ss.Asks != nil {
		return price > ss.Asks.Price
	}
	if side == constants.ORDER_SIDE_SELL && ss.Bids != nil {
		return price < ss.Bids.Price
	}
	return false
}

func (e *Engine) getAvailableQuantity(symbol types.SymbolID, side int8, maxPrice int64) types.Quantity {
	ss := e.state.GetSymbolState(symbol)
	var total types.Quantity
	if side == constants.ORDER_SIDE_BUY {
		for level := ss.Asks; level != nil; level = level.NextPriceLevel {
			if maxPrice == 0 || int64(level.Price) <= maxPrice {
				total += level.Quantity
			}
		}
	} else {
		for level := ss.Bids; level != nil; level = level.NextPriceLevel {
			if maxPrice == 0 || int64(level.Price) >= maxPrice {
				total += level.Quantity
			}
		}
	}
	return total
}

func (e *Engine) matchOrder(order *types.Order) types.Quantity {
	ss := e.state.GetSymbolState(order.Symbol)
	var filled types.Quantity
	if order.Side == constants.ORDER_SIDE_BUY {
		for order.Quantity > filled && ss.Asks != nil {
			filled += e.matchAtLevel(order, ss.Asks)
		}
	} else {
		for order.Quantity > filled && ss.Bids != nil {
			filled += e.matchAtLevel(order, ss.Bids)
		}
	}
	if filled > 0 {
		e.unlockBalanceForOrder(order)
	}
	return filled
}

func (e *Engine) matchAtLevel(order *types.Order, level *state.PriceLevel) types.Quantity {
	var filled types.Quantity
	tradePrice := level.Price
	category := e.getSymbolCategory(order.Symbol)

	for order.Quantity > filled {
		var oid types.OrderID
		for oid = range level.Orders {
			break
		}
		if oid == 0 {
			break
		}

		maker := level.Orders[oid]
		tradeQty := min(order.Quantity-filled, maker.Quantity-maker.Filled)

		if category == constants.CATEGORY_SPOT {
			e.executeSpotTrade(order, maker, tradePrice, tradeQty)
		} else {
			lev := e.getUserLeverage(order.UserID, order.Symbol)
			e.executeLinearTrade(order, maker, tradePrice, tradeQty, lev)
		}

		maker.Filled += tradeQty
		filled += tradeQty
		order.Filled += tradeQty

		if maker.Filled >= maker.Quantity {
			maker.Status = constants.ORDER_STATUS_FILLED
			delete(level.Orders, oid)
			level.Quantity -= tradeQty
		}

		if order.Filled >= order.Quantity {
			order.Status = constants.ORDER_STATUS_FILLED
			break
		}
	}

	if len(level.Orders) == 0 {
		ss := e.state.GetSymbolState(order.Symbol)
		if order.Side == constants.ORDER_SIDE_BUY {
			ss.Asks = level.NextPriceLevel
		} else {
			ss.Bids = level.NextPriceLevel
		}
	}

	return filled
}

func (e *Engine) executeSpotTrade(taker, maker *types.Order, price types.Price, qty types.Quantity) {
	buyer, seller := taker, maker
	if taker.Side == constants.ORDER_SIDE_SELL {
		buyer, seller = maker, taker
	}

	value := int64(qty) * int64(price)
	e.getOrCreateBalance(buyer.UserID, "USDT").Available -= value
	e.getOrCreateBalance(seller.UserID, "USDT").Available += value

	// WHAAT????? HARDCODE ASSET????
	asset := "BTC"
	e.getOrCreateBalance(buyer.UserID, asset).Available += int64(qty)
	e.getOrCreateBalance(seller.UserID, asset).Available -= int64(qty)
}

func (e *Engine) executeLinearTrade(taker, maker *types.Order, price types.Price, qty types.Quantity, leverage int8) {
	margin := int64(qty) * int64(price) * int64(100/leverage) / 100
	e.updatePosition(taker.UserID, taker.Symbol, taker.Side, price, qty, leverage)
	e.updatePosition(maker.UserID, maker.Symbol, maker.Side, price, qty, leverage)

	tBal := e.getOrCreateBalance(taker.UserID, "USDT")
	if taker.Side == constants.ORDER_SIDE_BUY {
		tBal.Margin += margin
	} else {
		tBal.Margin -= margin
	}

	mBal := e.getOrCreateBalance(maker.UserID, "USDT")
	if maker.Side == constants.ORDER_SIDE_BUY {
		mBal.Margin += margin
	} else {
		mBal.Margin -= margin
	}
}

func (e *Engine) updatePosition(userID types.UserID, symbol types.SymbolID, side int8, price types.Price, qty types.Quantity, leverage int8) {
	pos := e.state.GetUserState(userID).Positions[symbol]
	if pos == nil {
		pos = &types.Position{UserID: userID, Symbol: symbol, Size: 0, Side: -1, Leverage: leverage}
		e.state.GetUserState(userID).Positions[symbol] = pos
	}

	switch {
	case pos.Size == 0:
		pos.Size, pos.EntryPrice, pos.Leverage, pos.Side = qty, price, leverage, side
	case pos.Side == side:
		pos.Size += qty
		pos.EntryPrice = types.Price((int64(pos.EntryPrice)*int64(pos.Size-qty) + int64(price)*int64(qty)) / int64(pos.Size))
	default:
		if qty >= pos.Size {
			pnl := int64(price-pos.EntryPrice) * int64(pos.Size)
			if pos.Side == constants.ORDER_SIDE_SELL {
				pnl = -pnl
			}
			pos.RealizedPnl += pnl
			pos.Size, pos.Side, pos.EntryPrice = qty-pos.Size, side, price
		} else {
			pnl := int64(price-pos.EntryPrice) * int64(qty)
			if pos.Side == constants.ORDER_SIDE_SELL {
				pnl = -pnl
			}
			pos.RealizedPnl += pnl
			pos.Size -= qty
		}
	}

	if pos.Size == 0 {
		pos.Side = -1
		bal := e.getOrCreateBalance(userID, "USDT")
		bal.Available += bal.Margin
		bal.Margin = 0
	}
}

func (e *Engine) getOrCreateBalance(userID types.UserID, asset string) *types.UserBalance {
	us := e.state.GetUserState(userID)
	if bal := us.Balances[asset]; bal != nil {
		return bal
	}
	bal := &types.UserBalance{UserID: userID, Asset: asset}
	us.Balances[asset] = bal
	return bal
}

func (e *Engine) lockBalanceForOrder(order *types.Order) {
	if order.Type == constants.ORDER_TYPE_MARKET {
		return
	}

	category := e.getSymbolCategory(order.Symbol)
	bal := e.getOrCreateBalance(order.UserID, "USDT")
	toLock := int64(order.Quantity-order.Filled) * int64(order.Price)

	if category == constants.CATEGORY_SPOT {
		bal.Locked += toLock
	} else {
		margin := toLock * int64(100/e.getUserLeverage(order.UserID, order.Symbol)) / 100
		bal.Margin += margin
		bal.Locked += margin
	}
}

func (e *Engine) unlockBalanceForOrder(order *types.Order) {
	category := e.getSymbolCategory(order.Symbol)
	bal := e.getOrCreateBalance(order.UserID, "USDT")
	toUnlock := int64(order.Quantity-order.Filled) * int64(order.Price)

	if category == constants.CATEGORY_SPOT {
		bal.Locked -= toUnlock
	} else {
		margin := toUnlock * int64(100/e.getUserLeverage(order.UserID, order.Symbol)) / 100
		bal.Locked -= margin
	}

	if bal.Locked < 0 {
		bal.Locked = 0
	}
}

func (e *Engine) executeReduceOnly(order *types.Order, input *types.OrderInput, result *types.OrderResult, now time.Time) (*types.OrderResult, error) {
	pos := e.state.GetUserState(order.UserID).Positions[order.Symbol]
	if pos == nil || pos.Size == 0 {
		e.setStatus(order, result, constants.ORDER_STATUS_CANCELED)
		return result, nil
	}

	tradeQty := input.Quantity
	if tradeQty > pos.Size {
		tradeQty = pos.Size
	}

	execPrice := pos.EntryPrice
	if order.Type == constants.ORDER_TYPE_MARKET {
		if p := e.getCurrentPrice(order.Symbol); p > 0 {
			execPrice = p
		}
	} else {
		execPrice = order.Price
	}

	order.Filled = tradeQty
	result.Filled, result.Remaining = tradeQty, input.Quantity-tradeQty
	e.setStatus(order, result, constants.ORDER_STATUS_FILLED)

	e.updatePosition(order.UserID, order.Symbol, order.Side, execPrice, tradeQty, e.getUserLeverage(order.UserID, order.Symbol))
	e.logOrderOp(wal.OP_PLACE_ORDER, now, order.UserID, order.Symbol, order.ID)
	return result, nil
}

func (e *Engine) adjustLockedBalance(order *types.Order, oldQty types.Quantity, oldPrice types.Price) {
	bal := e.getOrCreateBalance(order.UserID, "USDT")
	bal.Locked += int64(order.Quantity)*int64(order.Price) - int64(oldQty)*int64(oldPrice)
}

func (e *Engine) setStatus(order *types.Order, result *types.OrderResult, status int8) {
	order.Status = status
	result.Status = status
}

func (e *Engine) getUserLeverage(userID types.UserID, symbol types.SymbolID) int8 {
	pos := e.state.GetUserState(userID).Positions[symbol]
	if pos != nil && pos.Leverage > 0 {
		return pos.Leverage
	}
	return 2
}

func (e *Engine) logOrderOp(opType wal.OperationType, now time.Time, userID types.UserID, symbol types.SymbolID, orderID types.OrderID) {
	_ = e.wal.Append(&wal.Operation{
		Type:      opType,
		Timestamp: now.Unix(),
		UserID:    int64(userID),
		Symbol:    int32(symbol),
		OrderID:   int64(orderID),
	})
}

func abs64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

func min(a, b types.Quantity) types.Quantity {
	if a < b {
		return a
	}
	return b
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func init() {
	_ = math.MaxInt64
}
