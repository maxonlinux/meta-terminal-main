package engine

import (
	"errors"
	"sync"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/history"
	"github.com/anomalyco/meta-terminal-go/internal/idgen"
	"github.com/anomalyco/meta-terminal-go/internal/market"
	"github.com/anomalyco/meta-terminal-go/internal/orderbook"
	"github.com/anomalyco/meta-terminal-go/internal/orderstore"
	"github.com/anomalyco/meta-terminal-go/internal/outbox"
	"github.com/anomalyco/meta-terminal-go/internal/pool"
	"github.com/anomalyco/meta-terminal-go/internal/registry"
	"github.com/anomalyco/meta-terminal-go/internal/safemath"
	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

var (
	ErrUnknownMarket       = errors.New("unknown market")
	ErrInvalidTIF          = errors.New("invalid time-in-force")
	ErrMarketTIF           = errors.New("market orders must be IOC or FOK")
	ErrLimitRequired       = errors.New("limit order required")
	ErrPostOnlyWouldCross  = errors.New("post-only would cross")
	ErrReduceOnlyQty       = errors.New("reduceOnly quantity exceeds position")
	ErrInsufficient        = errors.New("insufficient balance")
	ErrInsufficientBalance = errors.New("insufficient balance for new margin requirement")
	ErrPositionLiquidated  = errors.New("position would be liquidated with new leverage")
	ErrInvalidAmount       = errors.New("invalid amount")
)

type EventSink interface {
	OnOrderUpdate(order *types.Order)
	OnTrade(trade *types.Trade)
}

type Engine struct {
	mu sync.Mutex

	orderStore *orderstore.Store
	orderbooks *orderbook.State
	users      *state.Users
	registry   *registry.Registry
	triggers   *state.Triggers
	history    history.Reader
	outbox     *outbox.Outbox
	markets    map[int8]market.Market

	idGen *idgen.Snowflake
	sink  EventSink

	matchScratch []types.Match
}

type reserveChecker interface {
	CanReserve(userID types.UserID, symbol string, qty types.Quantity, price types.Price) bool
}

func New(
	orderStore *orderstore.Store,
	orderbooks *orderbook.State,
	users *state.Users,
	reg *registry.Registry,
	triggers *state.Triggers,
	hist history.Reader,
	out *outbox.Outbox,
	markets map[int8]market.Market,
	idGen *idgen.Snowflake,
) *Engine {
	return &Engine{
		orderStore:   orderStore,
		orderbooks:   orderbooks,
		users:        users,
		registry:     reg,
		triggers:     triggers,
		history:      hist,
		outbox:       out,
		markets:      markets,
		idGen:        idGen,
		matchScratch: make([]types.Match, 0, 8),
	}
}

func (e *Engine) nextID() types.OrderID {
	if e.idGen == nil {
		return types.OrderID(types.NowNano())
	}
	return types.OrderID(e.idGen.Next())
}

func (e *Engine) PlaceOrder(input *types.OrderInput) (*types.OrderResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	market := e.markets[input.Category]
	if market == nil {
		return nil, ErrUnknownMarket
	}

	return e.placeOrderWithIDLocked(e.nextID(), input, market)
}

func (e *Engine) PlaceOrderWithID(orderID types.OrderID, input *types.OrderInput) (*types.OrderResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	market := e.markets[input.Category]
	if market == nil {
		return nil, ErrUnknownMarket
	}
	if e.idGen != nil {
		e.idGen.AdvanceTo(uint64(orderID))
	}
	return e.placeOrderWithIDLocked(orderID, input, market)
}

func (e *Engine) placeOrderWithIDLocked(orderID types.OrderID, input *types.OrderInput, market market.Market) (*types.OrderResult, error) {
	if err := market.GetValidator().Validate(input); err != nil {
		return nil, err
	}

	if input.Type == constants.ORDER_TYPE_MARKET && input.TIF != constants.TIF_IOC && input.TIF != constants.TIF_FOK {
		return nil, ErrMarketTIF
	}

	if (input.TIF == constants.TIF_GTC || input.TIF == constants.TIF_POST_ONLY) && input.Type != constants.ORDER_TYPE_LIMIT {
		return nil, ErrLimitRequired
	}

	if input.Category == constants.CATEGORY_LINEAR && input.Leverage <= 0 {
		pos := e.users.GetPosition(input.UserID, input.Symbol)
		if pos.Leverage > 0 {
			input.Leverage = pos.Leverage
		} else {
			input.Leverage = 1
		}
	}

	order := pool.GetOrder()
	order.ID = orderID
	order.UserID = input.UserID
	order.Symbol = input.Symbol
	order.Category = input.Category
	order.Side = input.Side
	order.Type = input.Type
	order.TIF = input.TIF
	order.Status = constants.ORDER_STATUS_NEW
	order.Price = input.Price
	order.Quantity = input.Quantity
	order.TriggerPrice = input.TriggerPrice
	order.ReduceOnly = input.ReduceOnly
	order.CloseOnTrigger = input.CloseOnTrigger
	order.StopOrderType = input.StopOrderType
	order.Leverage = input.Leverage
	order.CreatedAt = types.NowNano()
	order.UpdatedAt = order.CreatedAt

	e.orderStore.Add(order)

	if input.TriggerPrice > 0 {
		order.Status = constants.ORDER_STATUS_UNTRIGGERED
		e.triggers.Get(order.Symbol).Add(order.ID, order.Side, order.TriggerPrice)
		e.emitOrder(order)
		result := pool.GetOrderResult()
		result.Order = order
		result.Status = order.Status
		result.Filled = order.Filled
		result.Remaining = order.Remaining()
		return result, nil
	}

	if input.ReduceOnly && input.Category == constants.CATEGORY_LINEAR {
		pos := e.users.GetPosition(input.UserID, input.Symbol)
		if pos.Size == 0 {
			return nil, ErrReduceOnlyQty
		}
		if order.Quantity > pos.Size {
			order.Quantity = pos.Size
		}
	}

	if order.TIF == constants.TIF_POST_ONLY {
		ob := market.GetOrderBookState().Get(order.Symbol, order.Category)
		if ob.WouldCross(order.Side, order.Price) {
			return nil, ErrPostOnlyWouldCross
		}
	}

	if order.TIF == constants.TIF_GTC || order.TIF == constants.TIF_POST_ONLY {
		if checker, ok := market.GetClearing().(reserveChecker); ok {
			qty, price := reserveParams(order, true)
			if !checker.CanReserve(order.UserID, order.Symbol, qty, price) {
				return nil, ErrInsufficient
			}
		}
	}

	return e.executeOrder(order, market)
}

func (e *Engine) executeOrder(order *types.Order, market market.Market) (*types.OrderResult, error) {
	ob := market.GetOrderBookState().Get(order.Symbol, order.Category)
	limitPrice := types.Price(0)
	if order.Type == constants.ORDER_TYPE_LIMIT {
		limitPrice = order.Price
	}

	if order.TIF == constants.TIF_FOK {
		available := ob.AvailableQuantity(order.Side, limitPrice, order.Quantity)
		if available < order.Quantity {
			order.Status = constants.ORDER_STATUS_CANCELED
			order.UpdatedAt = types.NowNano()
			result := e.finalizeOrder(order, nil)
			return result, nil
		}
	}

	var matches []types.Match
	if order.TIF != constants.TIF_POST_ONLY {
		matches = e.matchScratch[:0]
		var err error
		matches, err = ob.MatchInto(order, limitPrice, matches)
		if err != nil {
			return nil, err
		}
	}

	for _, match := range matches {
		market.GetClearing().ExecuteTrade(match.Trade, order, match.Maker)
		e.recordTrade(order, match.Maker, match.Trade)
		e.updateMakerStatus(match.Maker)
	}

	if order.TIF == constants.TIF_GTC || order.TIF == constants.TIF_POST_ONLY {
		if order.Remaining() > 0 {
			qty, price := reserveParams(order, false)
			if err := market.GetClearing().Reserve(order.UserID, order.Symbol, qty, price); err != nil {
				return nil, err
			}
			ob.AddResting(order)
		}
	}

	order.UpdatedAt = types.NowNano()
	order.Status = statusFor(order)

	result := e.finalizeOrder(order, matches)
	if matches != nil {
		e.matchScratch = matches[:0]
	}
	return result, nil
}

func (e *Engine) CancelOrder(orderID types.OrderID, userID types.UserID) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	order := e.orderStore.Get(userID, orderID)
	if order == nil || order.UserID != userID {
		return nil
	}

	if order.Status == constants.ORDER_STATUS_UNTRIGGERED {
		e.triggers.Get(order.Symbol).Remove(orderID)
	}

	if order.Status == constants.ORDER_STATUS_NEW || order.Status == constants.ORDER_STATUS_PARTIALLY_FILLED {
		market := e.markets[order.Category]
		if market != nil {
			qty, price := reserveParams(order, false)
			market.GetClearing().Release(order.UserID, order.Symbol, qty, price)
			market.GetOrderBookState().Get(order.Symbol, order.Category).RemoveResting(order.ID)
		}
	}

	order.Status = constants.ORDER_STATUS_CANCELED
	order.UpdatedAt = types.NowNano()
	e.emitOrder(order)
	e.orderStore.Remove(order.UserID, order.ID)
	e.recordClosedOrder(order)
	pool.PutOrder(order)

	return nil
}

func (e *Engine) OnPriceTick(symbol string, price types.Price) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.registry.SetLastPrice(symbol, price)
	for userID, user := range e.users.AllUsers() {
		pos := user.Positions[symbol]
		if pos == nil {
			continue
		}
		if pos.ShouldLiquidate(price) {
			_ = userID
		}
	}

	orderIDs := e.triggers.Get(symbol).Check(price)
	for _, orderID := range orderIDs {
		order := e.orderStore.GetByID(orderID)
		if order == nil {
			continue
		}
		if order.CloseOnTrigger {
			e.handleCloseOnTrigger(order)
		} else {
			e.handleConditional(order)
		}
		e.triggers.Get(symbol).Remove(orderID)
	}
}

func (e *Engine) handleConditional(order *types.Order) {
	market := e.markets[order.Category]
	if market == nil {
		return
	}
	order.TriggerPrice = 0
	order.CloseOnTrigger = false
	order.Status = constants.ORDER_STATUS_TRIGGERED
	order.UpdatedAt = types.NowNano()
	result, _ := e.executeOrder(order, market)
	e.ReleaseResult(result)
}

func (e *Engine) handleCloseOnTrigger(order *types.Order) {
	pos := e.users.GetPosition(order.UserID, order.Symbol)
	if pos.Size == 0 {
		order.Status = constants.ORDER_STATUS_TRIGGERED
		order.UpdatedAt = types.NowNano()
		return
	}

	order.TriggerPrice = 0
	order.CloseOnTrigger = false
	order.Status = constants.ORDER_STATUS_TRIGGERED
	if pos.Side == constants.SIDE_LONG {
		order.Side = constants.ORDER_SIDE_SELL
	} else {
		order.Side = constants.ORDER_SIDE_BUY
	}
	order.Quantity = pos.Size
	if order.Type == constants.ORDER_TYPE_LIMIT {
		order.ReduceOnly = true
	} else {
		order.ReduceOnly = false
	}

	market := e.markets[constants.CATEGORY_LINEAR]
	if market == nil {
		return
	}

	order.UpdatedAt = types.NowNano()
	result, _ := e.executeOrder(order, market)
	e.ReleaseResult(result)
}

func (e *Engine) recordTrade(taker *types.Order, maker *types.Order, trade *types.Trade) {
	if e.outbox != nil {
		_ = e.outbox.AppendTrade(trade)
	}
	e.emitTrade(trade)

	if taker.Category == constants.CATEGORY_LINEAR {
		e.adjustReduceOnlyOrdersLocked(taker.UserID, taker.Symbol)
	}
	if maker != nil && maker.Category == constants.CATEGORY_LINEAR {
		e.adjustReduceOnlyOrdersLocked(maker.UserID, maker.Symbol)
	}
}

func (e *Engine) recordClosedOrder(order *types.Order) {
	if e.outbox != nil {
		_ = e.outbox.AppendOrder(order)
	}
}

func (e *Engine) updateMakerStatus(maker *types.Order) {
	if maker == nil {
		return
	}
	if maker.Remaining() == 0 {
		maker.Status = constants.ORDER_STATUS_FILLED
		maker.UpdatedAt = types.NowNano()
		e.emitOrder(maker)
		e.orderStore.Remove(maker.UserID, maker.ID)
		e.recordClosedOrder(maker)
		pool.PutOrder(maker)
		return
	}
	if maker.Status == constants.ORDER_STATUS_NEW {
		maker.Status = constants.ORDER_STATUS_PARTIALLY_FILLED
		maker.UpdatedAt = types.NowNano()
		e.emitOrder(maker)
	}
}

func (e *Engine) adjustReduceOnlyOrdersLocked(userID types.UserID, symbol string) {
	pos := e.users.GetPosition(userID, symbol)
	if pos == nil {
		return
	}
	ids := e.orderStore.UserReduceOnlyOrders(userID)
	if len(ids) == 0 {
		return
	}

	total := types.Quantity(0)
	for i := range ids {
		order := e.orderStore.GetByID(ids[i])
		if order == nil || order.Symbol != symbol {
			continue
		}
		if order.Status != constants.ORDER_STATUS_NEW && order.Status != constants.ORDER_STATUS_PARTIALLY_FILLED && order.Status != constants.ORDER_STATUS_UNTRIGGERED {
			continue
		}
		remaining := order.Remaining()
		if remaining > 0 {
			total += remaining
		}
	}
	if total <= pos.Size {
		return
	}

	toAdjust := total - pos.Size
	for i := range ids {
		if toAdjust <= 0 {
			break
		}
		order := e.orderStore.GetByID(ids[i])
		if order == nil || order.Symbol != symbol {
			continue
		}
		if order.Status != constants.ORDER_STATUS_NEW && order.Status != constants.ORDER_STATUS_PARTIALLY_FILLED && order.Status != constants.ORDER_STATUS_UNTRIGGERED {
			continue
		}
		remaining := order.Remaining()
		if remaining <= 0 {
			continue
		}
		if order.Status == constants.ORDER_STATUS_UNTRIGGERED {
			e.cancelOrderLocked(order, 0)
			toAdjust -= remaining
			continue
		}
		if remaining <= toAdjust {
			e.cancelOrderLocked(order, 0)
			toAdjust -= remaining
			continue
		}

		newRemaining := remaining - toAdjust
		releaseQty, releasePrice := releaseParams(order, toAdjust)
		market := e.markets[order.Category]
		if market != nil {
			market.GetClearing().Release(order.UserID, order.Symbol, releaseQty, releasePrice)
		}
		order.Quantity = order.Filled + newRemaining
		ob := e.orderbooks.Get(order.Symbol, order.Category)
		ob.AdjustResting(order.ID, newRemaining)
		order.UpdatedAt = types.NowNano()
		e.emitOrder(order)
		toAdjust = 0
	}
}

func (e *Engine) cancelOrderLocked(order *types.Order, userID types.UserID) {
	if order == nil {
		return
	}
	if userID != 0 && order.UserID != userID {
		return
	}

	switch order.Status {
	case constants.ORDER_STATUS_UNTRIGGERED:
		e.triggers.Get(order.Symbol).Remove(order.ID)
	case constants.ORDER_STATUS_NEW, constants.ORDER_STATUS_PARTIALLY_FILLED:
		if order.TIF == constants.TIF_GTC || order.TIF == constants.TIF_POST_ONLY {
			if market := e.markets[order.Category]; market != nil {
				qty, price := reserveParams(order, false)
				market.GetClearing().Release(order.UserID, order.Symbol, qty, price)
			}
			e.orderbooks.Get(order.Symbol, order.Category).RemoveResting(order.ID)
		}
	default:
		return
	}

	order.Status = constants.ORDER_STATUS_CANCELED
	order.UpdatedAt = types.NowNano()
	e.emitOrder(order)
	e.orderStore.Remove(order.UserID, order.ID)
	e.recordClosedOrder(order)
	pool.PutOrder(order)
}

func (e *Engine) finalizeOrder(order *types.Order, matches []types.Match) *types.OrderResult {
	result := pool.GetOrderResult()
	result.Order = order
	result.Status = order.Status
	result.Filled = order.Filled
	result.Remaining = order.Remaining()

	if len(matches) > 0 {
		result.Trades = pool.GetTradeSlice(len(matches))
		for i := range matches {
			result.Trades = append(result.Trades, matches[i].Trade)
		}
	}

	e.emitOrder(order)

	if order.Status == constants.ORDER_STATUS_FILLED ||
		order.Status == constants.ORDER_STATUS_CANCELED ||
		order.Status == constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED {
		e.orderStore.Remove(order.UserID, order.ID)
		e.recordClosedOrder(order)
	}
	return result
}

func statusFor(order *types.Order) int8 {
	remaining := order.Remaining()
	switch order.TIF {
	case constants.TIF_GTC, constants.TIF_POST_ONLY:
		if remaining == 0 {
			return constants.ORDER_STATUS_FILLED
		}
		if remaining == order.Quantity {
			return constants.ORDER_STATUS_NEW
		}
		return constants.ORDER_STATUS_PARTIALLY_FILLED
	case constants.TIF_IOC:
		if remaining == 0 {
			return constants.ORDER_STATUS_FILLED
		}
		if remaining == order.Quantity {
			return constants.ORDER_STATUS_CANCELED
		}
		return constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED
	case constants.TIF_FOK:
		if remaining == 0 {
			return constants.ORDER_STATUS_FILLED
		}
		return constants.ORDER_STATUS_CANCELED
	default:
		return constants.ORDER_STATUS_CANCELED
	}
}

func reserveParams(order *types.Order, full bool) (types.Quantity, types.Price) {
	qty := order.Remaining()
	if full {
		qty = order.Quantity
	}
	price := order.Price
	if order.Category == constants.CATEGORY_SPOT {
		if order.Side == constants.ORDER_SIDE_SELL {
			qty = -qty
		}
		return qty, price
	}
	leverage := int64(order.Leverage)
	if leverage <= 0 {
		leverage = 1
	}
	adjusted := safemath.Div(int64(price), leverage)
	return qty, types.Price(adjusted)
}

func releaseParams(order *types.Order, qty types.Quantity) (types.Quantity, types.Price) {
	releaseQty := qty
	price := order.Price
	if order.Category == constants.CATEGORY_SPOT {
		if order.Side == constants.ORDER_SIDE_SELL {
			releaseQty = -qty
		}
		return releaseQty, price
	}
	leverage := int64(order.Leverage)
	if leverage <= 0 {
		leverage = 1
	}
	adjusted := safemath.Div(int64(price), leverage)
	return releaseQty, types.Price(adjusted)
}

func (e *Engine) SetEventSink(sink EventSink) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.sink = sink
}

func (e *Engine) emitOrder(order *types.Order) {
	if e.sink != nil {
		e.sink.OnOrderUpdate(order)
	}
}

func (e *Engine) emitTrade(trade *types.Trade) {
	if e.sink != nil {
		e.sink.OnTrade(trade)
	}
}

func (e *Engine) ReleaseResult(result *types.OrderResult) {
	if result == nil {
		return
	}
	for i := range result.Trades {
		if result.Trades[i] != nil {
			pool.PutTrade(result.Trades[i])
		}
	}
	pool.PutTradeSlice(result.Trades)
	result.Trades = nil
	if result.Order != nil {
		status := result.Order.Status
		if status == constants.ORDER_STATUS_FILLED ||
			status == constants.ORDER_STATUS_CANCELED ||
			status == constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED {
			pool.PutOrder(result.Order)
		}
	}
	result.Order = nil
	pool.PutOrderResult(result)
}

func (e *Engine) SetBalance(userID types.UserID, asset string, amount int64) error {
	if asset == "" {
		return ErrInvalidAmount
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	bal := e.users.GetBalance(userID, asset)
	bal.Restore([3]int64{amount, 0, 0})
	return nil
}

func (e *Engine) AddInstrument(symbol string, category int8, price types.Price) {
	inst := registry.FromSymbol(symbol)
	e.AddInstrumentFull(inst.Symbol, inst.BaseAsset, inst.QuoteAsset, category, price)
}

func (e *Engine) AddInstrumentFull(symbol, base, quote string, category int8, price types.Price) {
	e.mu.Lock()
	defer e.mu.Unlock()
	inst := &registry.Instrument{
		Symbol:     symbol,
		BaseAsset:  base,
		QuoteAsset: quote,
		Category:   category,
	}
	e.registry.SetInstrument(inst)
	if price > 0 {
		e.registry.SetLastPrice(symbol, price)
	}
}

var ErrInvalidLeverage = errors.New("invalid leverage")

func (e *Engine) SetLeverage(userID types.UserID, symbol string, leverage int8) error {
	if leverage <= 0 || leverage > 100 {
		return ErrInvalidLeverage
	}
	e.mu.Lock()
	defer e.mu.Unlock()

	pos := e.users.GetPosition(userID, symbol)
	if pos.Size == 0 {
		pos.SetLeverage(leverage)
		return nil
	}

	oldMargin := pos.InitialMargin
	newMargin := safemath.MulDiv(int64(pos.EntryPrice), int64(pos.Size), int64(leverage))
	marginDiff := newMargin - oldMargin

	inst := e.registry.GetInstrument(symbol)
	bal := e.users.GetBalance(userID, inst.QuoteAsset)
	if marginDiff > 0 && bal.Get(constants.BUCKET_AVAILABLE) < marginDiff {
		return ErrInsufficientBalance
	}

	current := types.Price(0)
	if last, ok := e.registry.LastPrice(symbol); ok && last > 0 {
		current = last
	} else {
		ob := e.orderbooks.Get(symbol, constants.CATEGORY_LINEAR)
		if bid, _, ok := ob.BestBid(); ok {
			current = bid
		} else if ask, _, ok := ob.BestAsk(); ok {
			current = ask
		}
	}
	if current > 0 {
		var upnl int64
		if pos.Side == constants.SIDE_LONG {
			upnl = safemath.Mul(int64(pos.Size), safemath.Sub(int64(current), int64(pos.EntryPrice)))
		} else if pos.Side == constants.SIDE_SHORT {
			upnl = safemath.Mul(int64(pos.Size), safemath.Sub(int64(pos.EntryPrice), int64(current)))
		}
		buffer := newMargin - newMargin/10
		if upnl < -buffer || upnl > buffer {
			return ErrPositionLiquidated
		}
	}

	pos.SetLeverage(leverage)

	if marginDiff > 0 {
		bal.Move(constants.BUCKET_AVAILABLE, constants.BUCKET_MARGIN, marginDiff)
	} else if marginDiff < 0 {
		bal.Move(constants.BUCKET_MARGIN, constants.BUCKET_AVAILABLE, -marginDiff)
	}
	return nil
}

type BalanceSnapshot struct {
	Asset     string
	Available int64
	Locked    int64
	Margin    int64
}

type PositionSnapshot struct {
	Symbol            string
	Size              types.Quantity
	Side              int8
	EntryPrice        types.Price
	Leverage          int8
	InitialMargin     int64
	MaintenanceMargin int64
	LiquidationPrice  types.Price
	Version           int64
}

func (e *Engine) OpenOrders(userID types.UserID) []*types.Order {
	e.mu.Lock()
	defer e.mu.Unlock()
	orders := e.orderStore.UserOrders(userID)
	out := make([]*types.Order, 0, len(orders))
	for _, order := range orders {
		if order == nil {
			continue
		}
		if order.Status == constants.ORDER_STATUS_NEW || order.Status == constants.ORDER_STATUS_PARTIALLY_FILLED || order.Status == constants.ORDER_STATUS_UNTRIGGERED {
			out = append(out, order)
		}
	}
	return out
}

func (e *Engine) Balances(userID types.UserID) []BalanceSnapshot {
	e.mu.Lock()
	defer e.mu.Unlock()
	user := e.users.Get(userID)
	out := make([]BalanceSnapshot, 0, len(user.Balances))
	for asset, bal := range user.Balances {
		b := bal.Snapshot()
		out = append(out, BalanceSnapshot{
			Asset:     asset,
			Available: b[0],
			Locked:    b[1],
			Margin:    b[2],
		})
	}
	return out
}

func (e *Engine) Balance(userID types.UserID, asset string) BalanceSnapshot {
	e.mu.Lock()
	defer e.mu.Unlock()
	bal := e.users.GetBalance(userID, asset)
	b := bal.Snapshot()
	return BalanceSnapshot{
		Asset:     asset,
		Available: b[0],
		Locked:    b[1],
		Margin:    b[2],
	}
}

func (e *Engine) Positions(userID types.UserID) []PositionSnapshot {
	e.mu.Lock()
	defer e.mu.Unlock()
	user := e.users.Get(userID)
	out := make([]PositionSnapshot, 0, len(user.Positions))
	for _, pos := range user.Positions {
		out = append(out, snapshotPosition(pos))
	}
	return out
}

func (e *Engine) Position(userID types.UserID, symbol string) PositionSnapshot {
	e.mu.Lock()
	defer e.mu.Unlock()
	pos := e.users.GetPosition(userID, symbol)
	return snapshotPosition(pos)
}

func (e *Engine) OrderHistory(userID types.UserID, symbol string, category int8, limit int) []types.Order {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.history == nil {
		return nil
	}
	return e.history.GetOrders(userID, symbol, category, limit)
}

func (e *Engine) TradeHistory(userID types.UserID, symbol string, category int8, limit int) []types.Trade {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.history == nil {
		return nil
	}
	return e.history.GetTrades(userID, symbol, category, limit)
}

func snapshotPosition(pos *positions.Position) PositionSnapshot {
	if pos == nil {
		return PositionSnapshot{}
	}
	return PositionSnapshot{
		Symbol:            pos.Symbol,
		Size:              pos.Size,
		Side:              pos.Side,
		EntryPrice:        pos.EntryPrice,
		Leverage:          pos.Leverage,
		InitialMargin:     pos.InitialMargin,
		MaintenanceMargin: pos.MaintenanceMargin,
		LiquidationPrice:  pos.LiquidationPrice,
		Version:           pos.Version,
	}
}

func (e *Engine) Snapshot() *snapshot.Snapshot {
	e.mu.Lock()
	defer e.mu.Unlock()

	snap := &snapshot.Snapshot{
		TakenAt: types.NowNano(),
	}
	if e.idGen != nil {
		last, seq := e.idGen.State()
		snap.IDGenLastMS = last
		snap.IDGenSeq = seq
	}

	instMap := e.registry.Instruments()
	if len(instMap) > 0 {
		snap.Instruments = make([]snapshot.Instrument, 0, len(instMap))
		for _, inst := range instMap {
			snap.Instruments = append(snap.Instruments, snapshot.Instrument{
				Symbol:     inst.Symbol,
				BaseAsset:  inst.BaseAsset,
				QuoteAsset: inst.QuoteAsset,
				Category:   inst.Category,
			})
		}
	}

	priceMap := e.registry.Prices()
	if len(priceMap) > 0 {
		snap.Prices = make([]snapshot.Price, 0, len(priceMap))
		for symbol, price := range priceMap {
			snap.Prices = append(snap.Prices, snapshot.Price{Symbol: symbol, Price: price})
		}
	}

	users := e.users.AllUsers()
	if len(users) > 0 {
		snap.Users = make([]snapshot.User, 0, len(users))
		for userID, user := range users {
			userSnap := snapshot.User{UserID: userID}
			if len(user.Balances) > 0 {
				userSnap.Balances = make([]snapshot.Balance, 0, len(user.Balances))
				for asset, bal := range user.Balances {
					userSnap.Balances = append(userSnap.Balances, snapshot.Balance{
						Asset:   asset,
						Buckets: bal.Snapshot(),
					})
				}
			}
			if len(user.Positions) > 0 {
				userSnap.Positions = make([]snapshot.Position, 0, len(user.Positions))
				for _, pos := range user.Positions {
					userSnap.Positions = append(userSnap.Positions, snapshot.Position{
						Symbol:            pos.Symbol,
						Size:              pos.Size,
						Side:              pos.Side,
						EntryPrice:        pos.EntryPrice,
						Leverage:          pos.Leverage,
						InitialMargin:     pos.InitialMargin,
						MaintenanceMargin: pos.MaintenanceMargin,
						LiquidationPrice:  pos.LiquidationPrice,
						Version:           pos.Version,
					})
				}
			}
			snap.Users = append(snap.Users, userSnap)
		}
	}

	orders := e.orderStore.All()
	if len(orders) > 0 {
		snap.Orders = make([]types.Order, 0, len(orders))
		for _, order := range orders {
			if order == nil {
				continue
			}
			snap.Orders = append(snap.Orders, *order)
		}
	}

	return snap
}

func (e *Engine) ApplySnapshot(snap *snapshot.Snapshot) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.orderStore.Reset()
	e.orderbooks.Reset()
	e.triggers.Reset()
	e.users.Reset()
	e.registry.Reset()

	if snap == nil {
		return
	}

	for _, inst := range snap.Instruments {
		e.registry.SetInstrument(&registry.Instrument{
			Symbol:     inst.Symbol,
			BaseAsset:  inst.BaseAsset,
			QuoteAsset: inst.QuoteAsset,
			Category:   inst.Category,
		})
	}
	for _, p := range snap.Prices {
		e.registry.SetLastPrice(p.Symbol, p.Price)
	}
	for _, user := range snap.Users {
		for _, bal := range user.Balances {
			b := e.users.GetBalance(user.UserID, bal.Asset)
			b.Restore(bal.Buckets)
		}
		for _, pos := range user.Positions {
			p := e.users.GetPosition(user.UserID, pos.Symbol)
			p.Symbol = pos.Symbol
			p.Size = pos.Size
			p.Side = pos.Side
			p.EntryPrice = pos.EntryPrice
			p.Leverage = pos.Leverage
			p.InitialMargin = pos.InitialMargin
			p.MaintenanceMargin = pos.MaintenanceMargin
			p.LiquidationPrice = pos.LiquidationPrice
			p.Version = pos.Version
		}
	}

	for i := range snap.Orders {
		order := snap.Orders[i]
		copyOrder := order
		e.orderStore.Add(&copyOrder)

		if order.Status == constants.ORDER_STATUS_UNTRIGGERED {
			e.triggers.Get(order.Symbol).Add(order.ID, order.Side, order.TriggerPrice)
			continue
		}
		if order.TIF == constants.TIF_GTC || order.TIF == constants.TIF_POST_ONLY {
			if order.Remaining() > 0 {
				e.orderbooks.Get(order.Symbol, order.Category).AddResting(&copyOrder)
			}
		}
	}

	if e.idGen != nil {
		e.idGen.Restore(snap.IDGenLastMS, snap.IDGenSeq)
	}
}
