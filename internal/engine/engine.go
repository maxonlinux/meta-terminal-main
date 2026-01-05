package engine

import (
	"errors"
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
	return &Engine{
		wal:     w,
		state:   s,
		monitor: trigger.NewMonitor(s),
	}
}

func (e *Engine) PlaceOrder(input *types.OrderInput) (*types.OrderResult, error) {
	now := time.Now()

	category := e.getSymbolCategory(input.Symbol)

	if input.ReduceOnly && category == constants.CATEGORY_SPOT {
		return nil, nil
	}

	if input.ReduceOnly {
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

	ss := e.state.GetSymbolState(input.Symbol)
	ss.OrderMap[orderID] = order

	result := &types.OrderResult{
		Order:  order,
		Trades: make([]*types.Trade, 0),
	}

	if input.TriggerPrice > 0 {
		order.Status = constants.ORDER_STATUS_UNTRIGGERED
		result.Status = constants.ORDER_STATUS_UNTRIGGERED
		e.monitor.AddOrder(order)
		e.wal.Append(&wal.Operation{
			Type:      wal.OP_PLACE_ORDER,
			Timestamp: now.Unix(),
			UserID:    int64(input.UserID),
			Symbol:    int32(input.Symbol),
			OrderID:   int64(orderID),
		})
		return result, nil
	}

	switch input.Type {
	case constants.ORDER_TYPE_MARKET:
		filled := e.matchOrder(order)
		order.Filled = filled
		if filled == 0 {
			order.Status = constants.ORDER_STATUS_CANCELED
			result.Status = constants.ORDER_STATUS_CANCELED
		} else if filled < order.Quantity {
			order.Status = constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED
			result.Status = constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED
		} else {
			order.Status = constants.ORDER_STATUS_FILLED
			result.Status = constants.ORDER_STATUS_FILLED
		}
		result.Filled = filled
		result.Remaining = order.Quantity - filled

	case constants.ORDER_TYPE_LIMIT:
		switch input.TIF {
		case constants.TIF_GTC, constants.TIF_POST_ONLY:
			if input.TIF == constants.TIF_POST_ONLY {
				wouldCross := e.checkWouldCross(input.Symbol, input.Side, input.Price)
				if wouldCross {
					order.Status = constants.ORDER_STATUS_CANCELED
					result.Status = constants.ORDER_STATUS_CANCELED
					e.wal.Append(&wal.Operation{
						Type:      wal.OP_CANCEL_ORDER,
						Timestamp: now.Unix(),
						UserID:    int64(input.UserID),
						Symbol:    int32(input.Symbol),
						OrderID:   int64(orderID),
					})
					return result, nil
				}
			}
			e.lockBalanceForOrder(order)
			order.Status = constants.ORDER_STATUS_NEW
			result.Status = constants.ORDER_STATUS_NEW
			e.addToOrderBook(order)

		case constants.TIF_IOC:
			filled := e.matchOrder(order)
			if filled == 0 {
				order.Status = constants.ORDER_STATUS_CANCELED
				result.Status = constants.ORDER_STATUS_CANCELED
			} else if filled < order.Quantity {
				result.Status = constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED
				order.Status = constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED
			} else {
				order.Status = constants.ORDER_STATUS_FILLED
				result.Status = constants.ORDER_STATUS_FILLED
			}
			result.Filled = filled
			result.Remaining = order.Quantity - filled

		case constants.TIF_FOK:
			filled := e.matchOrder(order)
			if filled < input.Quantity {
				order.Status = constants.ORDER_STATUS_CANCELED
				result.Status = constants.ORDER_STATUS_CANCELED
				result.Filled = 0
				result.Remaining = input.Quantity
				return result, nil
			}
			order.Status = constants.ORDER_STATUS_FILLED
			result.Status = constants.ORDER_STATUS_FILLED
			result.Filled = filled
			result.Remaining = 0
		}
	}

	e.wal.Append(&wal.Operation{
		Type:      wal.OP_PLACE_ORDER,
		Timestamp: now.Unix(),
		UserID:    int64(input.UserID),
		Symbol:    int32(input.Symbol),
		OrderID:   int64(orderID),
	})
	return result, nil
}

func (e *Engine) CancelOrder(orderID types.OrderID, userID types.UserID) error {
	order := e.getOrderByID(orderID)
	if order == nil {
		return nil
	}

	if userID != 0 && order.UserID != userID {
		return nil
	}

	switch order.Status {
	case constants.ORDER_STATUS_NEW:
		e.removeFromOrderBook(order)
		e.unlockBalanceForOrder(order)
		order.Status = constants.ORDER_STATUS_CANCELED

	case constants.ORDER_STATUS_UNTRIGGERED:
		e.monitor.RemoveOrder(orderID, order.Symbol)
		order.Status = constants.ORDER_STATUS_CANCELED

	case constants.ORDER_STATUS_PARTIALLY_FILLED:
		e.unlockBalanceForOrder(order)
		order.Status = constants.ORDER_STATUS_CANCELED
	}

	order.UpdatedAt = time.Now()
	e.wal.Append(&wal.Operation{
		Type:      wal.OP_CANCEL_ORDER,
		Timestamp: time.Now().Unix(),
		UserID:    int64(order.UserID),
		Symbol:    int32(order.Symbol),
		OrderID:   int64(orderID),
	})
	return nil
}

func (e *Engine) AmendOrder(orderID types.OrderID, userID types.UserID, newQuantity types.Quantity, newPrice types.Price) error {
	order := e.getOrderByID(orderID)
	if order == nil || order.UserID != userID {
		return nil
	}

	if order.Status != constants.ORDER_STATUS_NEW {
		return nil
	}

	oldPrice := order.Price
	oldQty := order.Quantity

	order.Price = newPrice
	order.Quantity = newQuantity
	order.UpdatedAt = time.Now()

	if oldPrice != newPrice {
		ss := e.state.GetSymbolState(order.Symbol)
		delete(ss.OrderMap, orderID)
		ss.OrderMap[orderID] = order
	}

	e.adjustLockedBalance(order, oldQty, oldPrice)

	e.wal.Append(&wal.Operation{
		Type:      wal.OP_AMEND_ORDER,
		Timestamp: time.Now().Unix(),
		UserID:    int64(userID),
		Symbol:    int32(order.Symbol),
		OrderID:   int64(orderID),
	})
	return nil
}

func (e *Engine) GetOrder(orderID types.OrderID) *types.Order {
	return e.getOrderByID(orderID)
}

func (e *Engine) InitSymbolCategory(symbol types.SymbolID, category int8) {
	ss := e.state.GetSymbolState(symbol)
	ss.Category = category
}

func (e *Engine) GetUserOrders(userID types.UserID) []*types.Order {
	orders := make([]*types.Order, 0, 16)
	for _, order := range e.state.OrderByID {
		if order.UserID == userID {
			orders = append(orders, order)
		}
	}
	return orders
}

func (e *Engine) GetUserBalances(userID types.UserID) []*types.UserBalance {
	us := e.state.GetUserState(userID)
	balances := make([]*types.UserBalance, 0, len(us.Balances))
	for _, balance := range us.Balances {
		balances = append(balances, balance)
	}
	return balances
}

func (e *Engine) GetUserBalanceByAsset(userID types.UserID, asset string) *types.UserBalance {
	us := e.state.GetUserState(userID)
	return us.Balances[asset]
}

func (e *Engine) GetUserPosition(userID types.UserID, symbol types.SymbolID) *types.Position {
	us := e.state.GetUserState(userID)
	return us.Positions[symbol]
}

func (e *Engine) GetOrderBook(symbol types.SymbolID, limit int) (bids [][]int64, asks [][]int64) {
	ss := e.state.GetSymbolState(symbol)

	pl := ss.Bids
	for i := 0; i < limit && pl != nil; i++ {
		bids = append(bids, []int64{int64(pl.Price), int64(pl.Quantity)})
		pl = pl.NextPriceLevel
	}

	pl = ss.Asks
	for i := 0; i < limit && pl != nil; i++ {
		asks = append(asks, []int64{int64(pl.Price), int64(pl.Quantity)})
		pl = pl.NextPriceLevel
	}

	return bids, asks
}

func (e *Engine) ClosePosition(userID types.UserID, symbol types.SymbolID) error {
	us := e.state.GetUserState(userID)
	pos := us.Positions[symbol]
	if pos == nil || pos.Size == 0 {
		return nil
	}

	closeSide := int8(constants.ORDER_SIDE_SELL)
	if pos.Side == constants.ORDER_SIDE_SELL {
		closeSide = int8(constants.ORDER_SIDE_BUY)
	}

	input := &types.OrderInput{
		UserID:         userID,
		Symbol:         symbol,
		Side:           closeSide,
		Type:           constants.ORDER_TYPE_MARKET,
		TIF:            constants.TIF_IOC,
		Quantity:       pos.Size,
		CloseOnTrigger: true,
		ReduceOnly:     true,
	}

	e.PlaceOrder(input)
	return nil
}

func (e *Engine) EditLeverage(userID types.UserID, symbol types.SymbolID, leverage int8) error {
	if leverage < 1 || leverage > 100 {
		return ErrInvalidLeverage
	}

	us := e.state.GetUserState(userID)
	pos := us.Positions[symbol]

	if pos == nil {
		pos = &types.Position{
			UserID:   userID,
			Symbol:   symbol,
			Size:     0,
			Side:     -1,
			Leverage: leverage,
		}
		us.Positions[symbol] = pos
		return nil
	}

	if pos.Size == 0 {
		pos.Leverage = leverage
		return nil
	}

	oldLeverage := int(pos.Leverage)
	if oldLeverage == 0 {
		oldLeverage = 2
	}

	positionValue := abs64(int64(pos.Size)) * int64(pos.EntryPrice)

	oldRequiredMargin := positionValue / int64(oldLeverage)
	newRequiredMargin := positionValue / int64(leverage)

	marginDiff := newRequiredMargin - oldRequiredMargin

	bal := e.getOrCreateBalance(userID, "USDT")

	if marginDiff > 0 {
		if bal.Available < marginDiff {
			return ErrInsufficientBalance
		}
		bal.Available -= marginDiff
		bal.Margin = newRequiredMargin
	} else if marginDiff < 0 {
		releasedMargin := -marginDiff
		bal.Available += releasedMargin
		bal.Margin = newRequiredMargin
	}

	liquidationPrice := e.calculateLiquidationPrice(pos, leverage)
	currentPrice := e.getCurrentPrice(symbol)
	if currentPrice > 0 && liquidationPrice > 0 {
		if pos.Side == constants.ORDER_SIDE_BUY && currentPrice <= liquidationPrice {
			return ErrPositionLiquidated
		}
		if pos.Side == constants.ORDER_SIDE_SELL && currentPrice >= liquidationPrice {
			return ErrPositionLiquidated
		}
	}

	pos.Leverage = leverage

	return nil
}

func (e *Engine) EditTPSL(userID types.UserID, symbol types.SymbolID, tpOrderID, slOrderID int64, tpPrice, slPrice types.Price) error {
	us := e.state.GetUserState(userID)
	pos := us.Positions[symbol]
	if pos == nil || pos.Size == 0 {
		return nil
	}

	tpSide := int8(constants.ORDER_SIDE_SELL)
	if pos.Side == constants.ORDER_SIDE_SELL {
		tpSide = int8(constants.ORDER_SIDE_BUY)
	}

	if tpOrderID == 0 && tpPrice > 0 {
		tpOrder := &types.Order{
			ID:             e.state.NextOrderID,
			UserID:         userID,
			Symbol:         symbol,
			Side:           tpSide,
			Type:           constants.ORDER_TYPE_LIMIT,
			TIF:            constants.TIF_GTC,
			Price:          tpPrice,
			Quantity:       pos.Size,
			Status:         constants.ORDER_STATUS_UNTRIGGERED,
			TriggerPrice:   tpPrice,
			StopOrderType:  constants.STOP_ORDER_TYPE_TP,
			CloseOnTrigger: true,
			ReduceOnly:     true,
		}
		e.state.NextOrderID++
		e.monitor.AddOrder(tpOrder)
	}

	if slOrderID == 0 && slPrice > 0 {
		slOrder := &types.Order{
			ID:             e.state.NextOrderID,
			UserID:         userID,
			Symbol:         symbol,
			Side:           tpSide,
			Type:           constants.ORDER_TYPE_LIMIT,
			TIF:            constants.TIF_GTC,
			Price:          slPrice,
			Quantity:       pos.Size,
			Status:         constants.ORDER_STATUS_UNTRIGGERED,
			TriggerPrice:   slPrice,
			StopOrderType:  constants.STOP_ORDER_TYPE_SL,
			CloseOnTrigger: true,
			ReduceOnly:     true,
		}
		e.state.NextOrderID++
		e.monitor.AddOrder(slOrder)
	}

	return nil
}

func (e *Engine) OnTrigger(order *types.Order) {
	order.Status = constants.ORDER_STATUS_TRIGGERED

	if order.CloseOnTrigger {
		us := e.state.GetUserState(order.UserID)
		pos := us.Positions[order.Symbol]
		if pos != nil && pos.Size != 0 {
			e.ClosePosition(order.UserID, order.Symbol)
		}
		return
	}

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
	e.state.NextOrderID++

	ss := e.state.GetSymbolState(order.Symbol)
	ss.OrderMap[newOrder.ID] = newOrder

	e.lockBalanceForOrder(newOrder)
	e.addToOrderBook(newOrder)
}

func (e *Engine) getOrderByID(orderID types.OrderID) *types.Order {
	for _, ss := range e.state.Symbols {
		if order, ok := ss.OrderMap[orderID]; ok {
			return order
		}
	}
	return nil
}

func (e *Engine) getSymbolCategory(symbol types.SymbolID) int8 {
	ss := e.state.GetSymbolState(symbol)
	return ss.Category
}

func (e *Engine) getUserLeverage(userID types.UserID, symbol types.SymbolID) int8 {
	us := e.state.GetUserState(userID)
	pos := us.Positions[symbol]
	if pos != nil && pos.Leverage > 0 {
		return pos.Leverage
	}
	return 2 // По умолчанию leverage = 2
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

func (e *Engine) calculateLiquidationPrice(pos *types.Position, newLeverage int8) types.Price {
	if pos.Size == 0 || newLeverage == 0 {
		return 0
	}

	marginRatio := float64(newLeverage) / 100.0
	liqDistance := float64(pos.EntryPrice) * marginRatio * 10

	if pos.Side == constants.ORDER_SIDE_BUY {
		return types.Price(float64(pos.EntryPrice) - liqDistance)
	}
	return types.Price(float64(pos.EntryPrice) + liqDistance)
}

func (e *Engine) validateReduceOnly(input *types.OrderInput) error {
	us := e.state.GetUserState(input.UserID)
	pos := us.Positions[input.Symbol]
	if pos == nil || pos.Size == 0 {
		return errors.New("reduceOnly order requires an existing position")
	}

	if input.Side == constants.ORDER_SIDE_BUY && pos.Side == constants.ORDER_SIDE_SELL {
		return nil
	}
	if input.Side == constants.ORDER_SIDE_SELL && pos.Side == constants.ORDER_SIDE_BUY {
		return nil
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
	}

	(*level).Quantity += order.Quantity
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

func (e *Engine) checkWouldCross(symbol types.SymbolID, side int8, price types.Price) bool {
	ss := e.state.GetSymbolState(symbol)

	if side == constants.ORDER_SIDE_BUY && ss.Asks != nil {
		return price >= ss.Asks.Price
	}
	if side == constants.ORDER_SIDE_SELL && ss.Bids != nil {
		return price <= ss.Bids.Price
	}
	return false
}

func (e *Engine) matchOrder(order *types.Order) types.Quantity {
	ss := e.state.GetSymbolState(order.Symbol)

	var filled types.Quantity
	if order.Side == constants.ORDER_SIDE_BUY {
		for order.Quantity > filled && ss.Asks != nil {
			filled = e.matchAtLevel(order, ss.Asks)
		}
	} else {
		for order.Quantity > filled && ss.Bids != nil {
			filled = e.matchAtLevel(order, ss.Bids)
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
			leverage := e.getUserLeverage(order.UserID, order.Symbol)
			e.executeLinearTrade(order, maker, tradePrice, tradeQty, leverage)
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
		if order.Side == constants.ORDER_SIDE_BUY {
			ss := e.state.GetSymbolState(order.Symbol)
			ss.Asks = level.NextPriceLevel
		} else {
			ss := e.state.GetSymbolState(order.Symbol)
			ss.Bids = level.NextPriceLevel
		}
	}

	return filled
}

func (e *Engine) executeSpotTrade(taker, maker *types.Order, price types.Price, qty types.Quantity) {
	buyer := taker
	seller := maker
	if taker.Side == constants.ORDER_SIDE_SELL {
		buyer = maker
		seller = taker
	}

	tradeValue := int64(qty) * int64(price)

	buyerBal := e.getOrCreateBalance(buyer.UserID, "USDT")
	buyerBal.Available -= tradeValue

	sellerBal := e.getOrCreateBalance(seller.UserID, "USDT")
	sellerBal.Available += tradeValue

	asset := e.getAssetForSymbol(taker.Symbol)
	buyerAssetBal := e.getOrCreateBalance(buyer.UserID, asset)
	buyerAssetBal.Available += int64(qty)

	sellerAssetBal := e.getOrCreateBalance(seller.UserID, asset)
	sellerAssetBal.Available -= int64(qty)
}

func (e *Engine) executeLinearTrade(taker, maker *types.Order, price types.Price, qty types.Quantity, leverage int8) {
	tradeValue := int64(qty) * int64(price)
	marginRatio := int64(leverage)
	if marginRatio > 0 {
		marginRatio = 100 / marginRatio
	}
	margin := tradeValue * marginRatio / 100

	e.updatePosition(taker.UserID, taker.Symbol, taker.Side, price, qty, leverage)
	e.updatePosition(maker.UserID, maker.Symbol, maker.Side, price, qty, leverage)

	takerBal := e.getOrCreateBalance(taker.UserID, "USDT")
	if taker.Side == constants.ORDER_SIDE_BUY {
		takerBal.Margin += margin
	} else {
		takerBal.Margin -= margin
	}

	makerBal := e.getOrCreateBalance(maker.UserID, "USDT")
	if maker.Side == constants.ORDER_SIDE_BUY {
		makerBal.Margin += margin
	} else {
		makerBal.Margin -= margin
	}
}

func (e *Engine) updatePosition(userID types.UserID, symbol types.SymbolID, side int8, price types.Price, qty types.Quantity, leverage int8) {
	us := e.state.GetUserState(userID)
	pos := us.Positions[symbol]
	if pos == nil {
		pos = &types.Position{
			UserID:   userID,
			Symbol:   symbol,
			Size:     0,
			Side:     -1,
			Leverage: leverage,
		}
		us.Positions[symbol] = pos
	}

	if pos.Size == 0 {
		pos.Size = qty
		pos.EntryPrice = price
		pos.Leverage = leverage
		pos.Side = side
	} else if pos.Side == side {
		pos.Size += qty
		newEntryPrice := types.Price((int64(pos.EntryPrice)*int64(pos.Size-qty) + int64(price)*int64(qty)) / int64(pos.Size))
		pos.EntryPrice = newEntryPrice
	} else {
		if qty >= pos.Size {
			closedSize := pos.Size
			pnl := int64(price-pos.EntryPrice) * int64(closedSize)
			if pos.Side == constants.ORDER_SIDE_SELL {
				pnl = -pnl
			}
			pos.RealizedPnl += pnl
			pos.Size = qty - pos.Size
			pos.Side = side
			pos.EntryPrice = price
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
	bal := us.Balances[asset]
	if bal == nil {
		bal = &types.UserBalance{
			UserID:    userID,
			Asset:     asset,
			Available: 0,
			Locked:    0,
			Margin:    0,
			Version:   0,
		}
		us.Balances[asset] = bal
	}
	return bal
}

func (e *Engine) getAssetForSymbol(symbol types.SymbolID) string {
	return "BTC"
}

func (e *Engine) lockBalanceForOrder(order *types.Order) {
	if order.Type == constants.ORDER_TYPE_MARKET {
		return
	}

	category := e.getSymbolCategory(order.Symbol)
	bal := e.getOrCreateBalance(order.UserID, "USDT")

	if category == constants.CATEGORY_SPOT {
		locked := int64(order.Quantity-order.Filled) * int64(order.Price)
		bal.Locked += locked
	} else {
		leverage := e.getUserLeverage(order.UserID, order.Symbol)
		marginRatio := int64(leverage)
		if marginRatio > 0 {
			marginRatio = 100 / marginRatio
		}
		margin := int64(order.Quantity-order.Filled) * int64(order.Price) * marginRatio / 100
		bal.Margin += margin
		bal.Locked += margin
	}
}

func (e *Engine) unlockBalanceForOrder(order *types.Order) {
	category := e.getSymbolCategory(order.Symbol)
	bal := e.getOrCreateBalance(order.UserID, "USDT")

	if category == constants.CATEGORY_SPOT {
		unlocked := int64(order.Quantity-order.Filled) * int64(order.Price)
		bal.Locked -= unlocked
	} else {
		leverage := e.getUserLeverage(order.UserID, order.Symbol)
		marginRatio := int64(leverage)
		if marginRatio > 0 {
			marginRatio = 100 / marginRatio
		}
		margin := int64(order.Quantity-order.Filled) * int64(order.Price) * marginRatio / 100
		bal.Locked -= margin
	}

	if bal.Locked < 0 {
		bal.Locked = 0
	}
}

func (e *Engine) adjustLockedBalance(order *types.Order, oldQty types.Quantity, oldPrice types.Price) {
	_ = e.getSymbolCategory(order.Symbol)
	bal := e.getOrCreateBalance(order.UserID, "USDT")

	oldLocked := int64(oldQty) * int64(oldPrice)
	newLocked := int64(order.Quantity) * int64(order.Price)

	bal.Locked += newLocked - oldLocked
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
