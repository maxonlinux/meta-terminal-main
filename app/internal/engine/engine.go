package engine

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/maxonlinux/meta-terminal-go/internal/clearing"
	"github.com/maxonlinux/meta-terminal-go/internal/oms"
	orderbook "github.com/maxonlinux/meta-terminal-go/internal/orderbook"
	"github.com/maxonlinux/meta-terminal-go/internal/portfolio"
	"github.com/maxonlinux/meta-terminal-go/internal/registry"
	"github.com/maxonlinux/meta-terminal-go/internal/trades"
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/events"
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/outbox"
	"github.com/maxonlinux/meta-terminal-go/pkg/snowflake"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/maxonlinux/meta-terminal-go/pkg/utils"
	"github.com/robaho/fixed"
)

type OrderCallback interface {
	OnChildOrderCreated(order *types.Order)
}

type PlanPolicy interface {
	CheckOrder(userID types.UserID, category int8, symbol string) error
	CheckLeverage(userID types.UserID, symbol string, leverage types.Leverage) error
}

type Command interface {
	Apply(*Engine, outbox.Writer) CommandResult
}

type CommandResult struct {
	Err     error
	Trades  []types.Trade
	Funding *types.FundingRequest
	Order   *types.Order
}

var (
	errInvalidOrderRequest = errors.New("invalid order request")
	errInvalidFundingID    = errors.New("invalid funding id")
	errOrderNotFound       = errors.New("order not found")
)

type Engine struct {
	store      *oms.Service
	outbox     *outbox.Outbox
	books      map[int8]map[string]*orderbook.OrderBook // category -> symbol -> orderbook
	clearing   *clearing.Service
	portfolio  *portfolio.Service
	tradeFeed  *trades.TradeFeed
	registry   *registry.Registry
	callback   OrderCallback
	publisher  EventPublisher
	planPolicy PlanPolicy
	locksMu    sync.Mutex
	locks      map[bookLockKey]*sync.Mutex // symbol & category -> mutex
	liqMu      sync.Mutex
	liqNext    map[liqKey]time.Time
}

func NewEngine(ob *outbox.Outbox, reg *registry.Registry, cb OrderCallback) (*Engine, error) {
	if reg == nil {
		return nil, fmt.Errorf("registry is required")
	}
	store := oms.NewService()
	e := &Engine{
		store:     store,
		outbox:    ob,
		books:     make(map[int8]map[string]*orderbook.OrderBook),
		tradeFeed: trades.NewTradeFeed(),
		registry:  reg,
		callback:  cb,
		locks:     make(map[bookLockKey]*sync.Mutex),
		liqNext:   make(map[liqKey]time.Time),
	}
	portfolioService, err := portfolio.New(func(userID types.UserID, symbol string, size types.Quantity) {
		e.onPositionReduce(userID, symbol, size)
	}, reg)
	if err != nil {
		return nil, err
	}
	clearingService, err := clearing.New(portfolioService, reg)
	if err != nil {
		return nil, err
	}

	e.clearing = clearingService
	e.portfolio = portfolioService
	portfolioService.OnBalanceUpdate(e.onBalanceUpdated)
	portfolioService.OnRealizedPnL(func(event types.RealizedPnL) {
		// Persist realized PnL as an outbox event for history.
		if e.outbox == nil {
			return
		}
		tx := e.outbox.Begin()
		if tx == nil {
			return
		}
		_ = tx.Record(events.EncodeRPNL(events.RPNLEvent{
			UserID:    event.UserID,
			OrderID:   event.OrderID,
			Symbol:    event.Symbol,
			Category:  event.Category,
			Side:      event.Side,
			Price:     event.Price,
			Quantity:  event.Quantity,
			Realized:  event.Realized,
			Timestamp: event.Timestamp,
		}))
		_ = tx.Commit()
	})

	for _, cat := range []int8{constants.CATEGORY_SPOT, constants.CATEGORY_LINEAR} {
		e.books[cat] = make(map[string]*orderbook.OrderBook)
	}

	return e, nil
}

func (e *Engine) SetPlanPolicy(policy PlanPolicy) {
	e.planPolicy = policy
}

func (e *Engine) SetPublisher(publisher EventPublisher) {
	e.publisher = publisher
}

func (e *Engine) Shutdown() {
	if e == nil {
		return
	}
	if e.outbox != nil {
		e.outbox.Stop()
	}
}

func (e *Engine) Registry() *registry.Registry {
	return e.registry
}

func (e *Engine) Portfolio() *portfolio.Service {
	return e.portfolio
}

func (e *Engine) Clearing() *clearing.Service {
	return e.clearing
}

func (e *Engine) Store() *oms.Service {
	return e.store
}

func (e *Engine) TradeFeed() *trades.TradeFeed {
	return e.tradeFeed
}

func (e *Engine) ReadBook(category int8, symbol string) *orderbook.OrderBook {
	lock := e.bookLock(symbol, category)
	lock.Lock()
	defer lock.Unlock()

	bookSet, ok := e.books[category]
	if !ok {
		return nil
	}
	if book, ok := bookSet[symbol]; ok {
		return book
	}
	if e.registry.GetInstrument(symbol) == nil {
		return nil
	}
	book := orderbook.New()
	bookSet[symbol] = book
	return book
}

func (e *Engine) Cmd(cmd Command) CommandResult {
	var tx *outbox.Tx
	if e.outbox != nil {
		tx = e.outbox.Begin()
	}

	apply := func(symbol string, category int8) CommandResult {
		return e.withBookLock(symbol, category, func() CommandResult {
			return cmd.Apply(e, tx)
		})
	}

	var result CommandResult
	switch c := cmd.(type) {
	case *PlaceOrderCmd:
		if c.Req != nil {
			result = apply(c.Req.Symbol, c.Req.Category)
		} else {
			result = CommandResult{Err: errInvalidOrderRequest}
		}
	case *CancelOrderCmd:
		if order, ok := e.store.GetUserOrder(c.UserID, c.OrderID); ok {
			result = e.withBookLock(order.Symbol, order.Category, func() CommandResult {
				if _, ok := e.store.GetUserOrder(c.UserID, c.OrderID); !ok {
					return CommandResult{Err: errOrderNotFound}
				}
				return cmd.Apply(e, tx)
			})
		} else {
			result = CommandResult{Err: errOrderNotFound}
		}
	case *AmendOrderCmd:
		if order, ok := e.store.GetUserOrder(c.UserID, c.OrderID); ok {
			result = e.withBookLock(order.Symbol, order.Category, func() CommandResult {
				if _, ok := e.store.GetUserOrder(c.UserID, c.OrderID); !ok {
					return CommandResult{Err: errOrderNotFound}
				}
				return cmd.Apply(e, tx)
			})
		} else {
			result = CommandResult{Err: errOrderNotFound}
		}
	case *SetLeverageCmd:
		if c.Symbol != "" {
			result = apply(c.Symbol, constants.CATEGORY_LINEAR)
		} else {
			result = CommandResult{Err: errInvalidOrderRequest}
		}
	default:
		result = cmd.Apply(e, tx)
	}
	if tx != nil {
		if result.Err != nil {
			_ = tx.Abort()
		} else {
			_ = tx.Commit()
		}
	}
	if result.Err == nil && result.Order != nil && e.publisher != nil {
		e.publisher.OnOrderUpdated(result.Order)
	}
	return result
}

func (e *Engine) checkLeverage(userID types.UserID, symbol string, leverage types.Leverage, price types.Price) error {
	pos := e.portfolio.GetPosition(userID, symbol)
	if math.Sign(pos.Size) == 0 {
		return nil
	}

	effective := leverage
	if math.Sign(effective) <= 0 {
		effective = types.Leverage(fixed.NewI(int64(constants.DEFAULT_LEVERAGE), 0))
	}
	liqPrice := clearing.LiquidationPrice(pos.EntryPrice, effective, pos.Size)
	if clearing.ShouldLiquidate(price, liqPrice, pos.Size) {
		return constants.ErrLeverageTooHigh
	}
	return nil
}

func (e *Engine) checkLinearOrderLeverage(userID types.UserID, symbol string, side int8) error {
	pos := e.portfolio.GetPosition(userID, symbol)
	if pos == nil {
		return nil
	}

	if math.Sign(pos.Size) > 0 && side == constants.ORDER_SIDE_SELL {
		return nil
	}
	if math.Sign(pos.Size) < 0 && side == constants.ORDER_SIDE_BUY {
		return nil
	}

	leverage := pos.Leverage
	if math.Sign(leverage) <= 0 {
		leverage = types.Leverage(fixed.NewI(int64(constants.DEFAULT_LEVERAGE), 0))
	}

	if clearing.IsImmediateLiquidationLeverage(leverage) {
		return constants.ErrLeverageTooHigh
	}

	return nil
}

func (e *Engine) onPositionReduce(userID types.UserID, symbol string, size types.Quantity) {
	price := types.Price{}
	if tick, ok := e.registry.GetPrice(symbol); ok {
		price = tick.Price
	}
	e.store.OnPositionReduce(userID, symbol, size, price)
	if math.Sign(size) == 0 {
		var buf [16]types.OrderID
		ids := buf[:0]
		e.store.Iterate(func(order *types.Order) bool {
			if order == nil {
				return true
			}
			if order.UserID != userID || order.Symbol != symbol {
				return true
			}
			if !order.CloseOnTrigger || !order.ReduceOnly {
				return true
			}
			switch order.StopOrderType {
			case constants.STOP_ORDER_TYPE_TAKE_PROFIT, constants.STOP_ORDER_TYPE_STOP_LOSS:
				ids = append(ids, order.ID)
			}
			return true
		})
		for _, id := range ids {
			order, ok := e.store.GetUserOrder(userID, id)
			if !ok {
				continue
			}
			_ = e.store.Cancel(userID, id)
			if e.publisher != nil {
				e.publisher.OnOrderUpdated(order)
			}
		}
	}
}

func (e *Engine) onBalanceUpdated(userID types.UserID, asset string, balance *types.Balance) {
	if e.publisher != nil {
		e.publisher.OnBalanceUpdated(userID, asset, balance)
	}
}

func (e *Engine) OnPriceTick(symbol string, price types.Price) {
	e.registry.SetPrice(symbol, registry.PriceTick{Price: price, Timestamp: utils.NowNano()})
	var tx *outbox.Tx
	var recorded bool
	e.store.OnPriceTick(symbol, price, func(order *types.Order) {
		if tx == nil && e.outbox != nil {
			tx = e.outbox.Begin()
		}
		if tx != nil {
			recorded = true
			_ = tx.Record(events.EncodeOrderTriggered(events.OrderTriggeredEvent{UserID: order.UserID, OrderID: order.ID, Timestamp: order.UpdatedAt}))
		}
		if tx != nil {
			recorded = true
		}
		_ = e.activateConditional(order, tx)
		if e.publisher != nil {
			e.publisher.OnOrderUpdated(order)
		}
		if e.callback != nil {
			e.callback.OnChildOrderCreated(order)
		}
	})
	_ = e.withBookLock(symbol, constants.CATEGORY_LINEAR, func() CommandResult {
		if tx != nil {
			recorded = true
		}
		e.checkLiquidations(symbol, price, tx)
		return CommandResult{}
	})
	if tx != nil {
		if recorded {
			_ = tx.Commit()
		} else {
			_ = tx.Abort()
		}
	}
}

type bookLockKey struct {
	symbol   string
	category int8
}

type liqKey struct {
	userID   types.UserID
	symbol   string
	category int8
}

func (e *Engine) withBookLock(symbol string, category int8, fn func() CommandResult) CommandResult {
	lock := e.bookLock(symbol, category)
	lock.Lock()
	defer lock.Unlock()
	return fn()
}

func (e *Engine) bookLock(symbol string, category int8) *sync.Mutex {
	key := bookLockKey{symbol: symbol, category: category}
	e.locksMu.Lock()
	defer e.locksMu.Unlock()
	lock := e.locks[key]
	if lock == nil {
		lock = &sync.Mutex{}
		e.locks[key] = lock
	}
	return lock
}

func (e *Engine) RebuildBooks() {
	// rebuild books from active limit orders
	e.store.Iterate(func(order *types.Order) bool {
		if order.IsConditional {
			return true
		}
		if order.Type != constants.ORDER_TYPE_LIMIT {
			return true
		}
		if order.Status != constants.ORDER_STATUS_NEW && order.Status != constants.ORDER_STATUS_PARTIALLY_FILLED {
			return true
		}
		remaining := math.Sub(order.Quantity, order.Filled)
		if math.Sign(remaining) <= 0 {
			return true
		}
		book, err := e.getBook(order.Category, order.Symbol)
		if err == nil {
			book.Add(order)
		}
		return true
	})
}

type PlaceOrderCmd struct {
	Req *types.PlaceOrderRequest
}

func (c *PlaceOrderCmd) Apply(e *Engine, writer outbox.Writer) CommandResult {
	req := c.Req
	if req == nil {
		return CommandResult{Err: errInvalidOrderRequest}
	}
	if req.Quantity.Sign() <= 0 {
		return CommandResult{Err: constants.ErrInvalidQuantity}
	}

	if req.Category != constants.CATEGORY_SPOT && req.Category != constants.CATEGORY_LINEAR {
		return CommandResult{Err: constants.ErrInvalidCategory}
	}

	if inst := e.registry.GetInstrument(req.Symbol); inst != nil {
		if math.Cmp(req.Quantity, inst.MinQty) < 0 {
			return CommandResult{Err: constants.ErrQtyOutOfBounds}
		}
		req.Price = types.Price(math.RoundTo(req.Price, inst.TickSize))
		req.Quantity = types.Quantity(math.RoundTo(req.Quantity, inst.StepSize))
	} else if e.registry != nil {
		return CommandResult{Err: constants.ErrInstrumentNotFound}
	}

	if e.planPolicy != nil && req.Origin == constants.ORDER_ORIGIN_USER {
		if err := e.planPolicy.CheckOrder(req.UserID, req.Category, req.Symbol); err != nil {
			return CommandResult{Err: err}
		}
	}

	if req.Category == constants.CATEGORY_LINEAR && req.Origin == constants.ORDER_ORIGIN_USER && !req.ReduceOnly {
		if err := e.checkLinearOrderLeverage(req.UserID, req.Symbol, req.Side); err != nil {
			return CommandResult{Err: err}
		}
	}

	if !req.TriggerPrice.IsZero() && req.Type == constants.ORDER_TYPE_LIMIT {
		if req.Category == constants.CATEGORY_SPOT {
			return CommandResult{Err: constants.ErrConditionalSpot}
		}

		switch req.Side {
		case constants.ORDER_SIDE_BUY:
			if math.Cmp(req.TriggerPrice, req.Price) > 0 {
				return CommandResult{Err: constants.ErrInvalidTriggerForBuy}
			}
		case constants.ORDER_SIDE_SELL:
			if math.Cmp(req.TriggerPrice, req.Price) < 0 {
				return CommandResult{Err: constants.ErrInvalidTriggerForSell}
			}
		}
	}

	if req.ReduceOnly && req.Category == constants.CATEGORY_SPOT {
		return CommandResult{Err: constants.ErrReduceOnlySpot}
	}

	order := e.store.Build(
		req.UserID,
		req.Symbol,
		req.Category,
		req.Origin,
		req.Side,
		req.Type,
		req.TIF,
		req.Price,
		req.Quantity,
		req.TriggerPrice,
		req.ReduceOnly,
		req.CloseOnTrigger,
		req.StopOrderType,
		triggerDirectionForOrder(req.StopOrderType, req.Side, req.TriggerPrice),
	)

	if order.IsConditional {
		e.store.Add(order)
		if writer != nil {
			_ = writer.Record(events.EncodeOrderPlaced(events.OrderPlacedEvent{
				Order:      order,
				Instrument: e.registry.GetInstrument(order.Symbol),
			}))
		}
		return CommandResult{Order: order}
	}

	book, err := e.getBook(req.Category, req.Symbol)
	if err != nil {
		return CommandResult{Err: err}
	}

	return e.executeOrder(order, book, writer, true)
}

func triggerDirectionForOrder(stopOrderType int8, side int8, triggerPrice types.Price) int8 {
	if triggerPrice.IsZero() {
		return constants.TRIGGER_DIRECTION_NONE
	}

	switch stopOrderType {
	case constants.STOP_ORDER_TYPE_TAKE_PROFIT:
		if side == constants.ORDER_SIDE_SELL {
			return constants.TRIGGER_DIRECTION_UP
		}
		return constants.TRIGGER_DIRECTION_DOWN
	case constants.STOP_ORDER_TYPE_STOP_LOSS, constants.STOP_ORDER_TYPE_STOP, constants.STOP_ORDER_TYPE_TRAILING:
		if side == constants.ORDER_SIDE_SELL {
			return constants.TRIGGER_DIRECTION_DOWN
		}
		return constants.TRIGGER_DIRECTION_UP
	default:
		if side == constants.ORDER_SIDE_BUY {
			return constants.TRIGGER_DIRECTION_DOWN
		}
		return constants.TRIGGER_DIRECTION_UP
	}
}

func hasSelfMatch(matches []types.Match, userID types.UserID) bool {
	for i := range matches {
		maker := matches[i].MakerOrder
		if maker != nil && maker.UserID == userID {
			return true
		}
	}
	return false
}

func (e *Engine) executeOrder(order *types.Order, book *orderbook.OrderBook, writer outbox.Writer, persist bool) CommandResult {
	limitPrice := order.Price
	if order.Type == constants.ORDER_TYPE_MARKET {
		limitPrice = types.Price{}
	}

	var buf [8]types.Match
	matches := book.GetMatchesUnsafe(order, limitPrice, buf[:0])
	if hasSelfMatch(matches, order.UserID) {
		return CommandResult{Err: constants.ErrSelfMatch}
	}
	if order.TIF == constants.TIF_POST_ONLY && len(matches) > 0 {
		return CommandResult{Err: constants.ErrPostOnlyWouldMatch}
	}
	var matchQty types.Quantity
	var matchNotional types.Quantity
	for i := range matches {
		matchQty = math.Add(matchQty, matches[i].Quantity)
		matchNotional = math.Add(matchNotional, math.Mul(matches[i].Price, matches[i].Quantity))
	}
	remaining := math.Sub(order.Quantity, matchQty)

	if order.TIF == constants.TIF_FOK && math.Sign(remaining) > 0 {
		return CommandResult{Err: constants.ErrFOKInsufficientLiquidity}
	}

	if math.Sign(matchQty) > 0 {
		avgPrice := types.Price(math.Div(matchNotional, matchQty))
		if err := e.clearing.Reserve(order.UserID, order.Symbol, order.Category, order.Side, matchQty, avgPrice); err != nil {
			return CommandResult{Err: err}
		}
	}

	if math.Sign(remaining) > 0 && (order.TIF == constants.TIF_POST_ONLY || order.TIF == constants.TIF_GTC) {
		if err := e.clearing.Reserve(order.UserID, order.Symbol, order.Category, order.Side, remaining, order.Price); err != nil {
			return CommandResult{Err: err}
		}
	}

	if persist {
		e.store.Add(order)
		if writer != nil {
			_ = writer.Record(events.EncodeOrderPlaced(events.OrderPlacedEvent{
				Order:      order,
				Instrument: e.registry.GetInstrument(order.Symbol),
			}))
		}
	}

	for i := range matches {
		match := matches[i]
		match.ID = types.TradeID(snowflake.Next())
		match.Timestamp = utils.NowNano()
		if err := e.applyTrade(book, match, writer); err != nil {
			return CommandResult{Err: err}
		}
	}

	if order.TIF == constants.TIF_IOC || order.Type == constants.ORDER_TYPE_MARKET {
		if math.Cmp(order.Filled, order.Quantity) != 0 {
			_ = e.store.Cancel(order.UserID, order.ID)
			return CommandResult{Order: order}
		}
		return CommandResult{Order: order}
	}

	if order.TIF == constants.TIF_POST_ONLY {
		book.AddUnsafe(order)
		if e.publisher != nil {
			e.publisher.OnOrderbookUpdated(order.Category, order.Symbol)
		}
		return CommandResult{Order: order}
	}

	if math.Sign(remaining) > 0 {
		book.AddUnsafe(order)
		if e.publisher != nil {
			e.publisher.OnOrderbookUpdated(order.Category, order.Symbol)
		}
	}
	return CommandResult{Order: order}
}

type CancelOrderCmd struct {
	UserID  types.UserID
	OrderID types.OrderID
}

func (c *CancelOrderCmd) Apply(e *Engine, writer outbox.Writer) CommandResult {
	if c.OrderID == 0 || c.UserID == 0 {
		return CommandResult{Err: errInvalidOrderRequest}
	}

	order, err := e.store.CancelWithDetails(c.UserID, c.OrderID)
	if err != nil {
		return CommandResult{Err: err}
	}
	if order.IsConditional {
		if writer != nil {
			_ = writer.Record(events.EncodeOrderCanceled(events.OrderCanceledEvent{UserID: order.UserID, OrderID: order.ID, Timestamp: order.UpdatedAt}))
		}
		return CommandResult{Order: order}
	}
	remaining := math.Sub(order.Quantity, order.Filled)
	if math.Sign(remaining) > 0 {
		if err := e.clearing.Release(order.UserID, order.Symbol, order.Category, order.Side, remaining, order.Price); err != nil {
			return CommandResult{Err: err}
		}
	}
	if book, err := e.getBook(order.Category, order.Symbol); err == nil {
		_ = book.RemoveUnsafe(order.ID)
	}
	if e.publisher != nil {
		e.publisher.OnOrderbookUpdated(order.Category, order.Symbol)
	}
	if writer != nil {
		_ = writer.Record(events.EncodeOrderCanceled(events.OrderCanceledEvent{UserID: order.UserID, OrderID: order.ID, Timestamp: order.UpdatedAt}))
	}
	return CommandResult{Order: order}
}

type AmendOrderCmd struct {
	UserID   types.UserID
	OrderID  types.OrderID
	NewQty   types.Quantity
	NewPrice types.Price
}

func (c *AmendOrderCmd) Apply(e *Engine, writer outbox.Writer) CommandResult {
	if c.OrderID == 0 || c.UserID == 0 {
		return CommandResult{Err: errInvalidOrderRequest}
	}
	order, ok := e.store.GetUserOrder(c.UserID, c.OrderID)
	if !ok {
		return CommandResult{Err: errOrderNotFound}
	}
	if math.Sign(c.NewPrice) > 0 && order.Origin != constants.ORDER_ORIGIN_SYSTEM {
		return CommandResult{Err: errInvalidOrderRequest}
	}
	newPrice := order.Price
	if math.Sign(c.NewPrice) > 0 {
		newPrice = c.NewPrice
	}

	oldRemaining := math.Sub(order.Quantity, order.Filled)
	newRemaining := math.Sub(c.NewQty, order.Filled)
	var reserveAsset string
	var reserveDelta types.Quantity
	remainDelta := math.Sub(newRemaining, oldRemaining)
	var book *orderbook.OrderBook
	if !order.IsConditional && order.Type == constants.ORDER_TYPE_LIMIT {
		if math.Sign(c.NewPrice) > 0 || math.Sign(remainDelta) != 0 {
			if b, err := e.getBook(order.Category, order.Symbol); err == nil {
				book = b
				_ = book.RemoveUnsafe(order.ID)
			}
		}
		pos := e.portfolio.GetPosition(order.UserID, order.Symbol)
		leverage := pos.Leverage
		if math.Sign(leverage) <= 0 {
			leverage = types.Leverage(fixed.NewI(int64(constants.DEFAULT_LEVERAGE), 0))
		}
		oldAmount, oldAsset, oldErr := clearing.CalculateReserveAmount(order.Symbol, order.Category, order.Side, oldRemaining, order.Price, leverage, e.registry)
		if oldErr != nil {
			return CommandResult{Err: oldErr}
		}
		newAmount, newAsset, newErr := clearing.CalculateReserveAmount(order.Symbol, order.Category, order.Side, newRemaining, newPrice, leverage, e.registry)
		if newErr != nil {
			return CommandResult{Err: newErr}
		}
		if oldAsset != newAsset {
			return CommandResult{Err: errInvalidOrderRequest}
		}
		reserveAsset = oldAsset
		reserveDelta = types.Quantity(math.Sub(newAmount, oldAmount))
		if math.Sign(reserveDelta) > 0 {
			if err := e.portfolio.Reserve(order.UserID, reserveAsset, reserveDelta); err != nil {
				return CommandResult{Err: err}
			}
		}
	}

	if err := e.store.Amend(c.UserID, c.OrderID, c.NewQty, c.NewPrice); err != nil {
		if book != nil {
			book.AddUnsafe(order)
		}
		if math.Sign(reserveDelta) > 0 {
			e.portfolio.Release(order.UserID, reserveAsset, reserveDelta)
		}
		return CommandResult{Err: err}
	}
	if writer != nil {
		_ = writer.Record(events.EncodeOrderAmended(events.OrderAmendedEvent{UserID: c.UserID, OrderID: c.OrderID, NewQty: c.NewQty, NewPrice: c.NewPrice, Timestamp: order.UpdatedAt}))
	}
	order, ok = e.store.GetUserOrder(c.UserID, c.OrderID)
	if !ok {
		return CommandResult{}
	}
	if !order.IsConditional && order.Type == constants.ORDER_TYPE_LIMIT {
		if math.Sign(reserveDelta) < 0 {
			e.portfolio.Release(order.UserID, reserveAsset, types.Quantity(math.Neg(reserveDelta)))
		}
		if book != nil {
			if math.Sign(newRemaining) > 0 {
				book.AddUnsafe(order)
			}
		}
		if e.publisher != nil {
			e.publisher.OnOrderbookUpdated(order.Category, order.Symbol)
		}
	}
	return CommandResult{Order: order}
}

type SetLeverageCmd struct {
	UserID   types.UserID
	Symbol   string
	Leverage types.Leverage
}

func (c *SetLeverageCmd) Apply(e *Engine, writer outbox.Writer) CommandResult {
	price := types.Price{}
	if tick, ok := e.registry.GetPrice(c.Symbol); ok {
		price = tick.Price
	}
	if price.Sign() <= 0 {
		return CommandResult{Err: constants.ErrPriceUnavailable}
	}
	if e.planPolicy != nil {
		if err := e.planPolicy.CheckLeverage(c.UserID, c.Symbol, c.Leverage); err != nil {
			return CommandResult{Err: err}
		}
	}
	if err := e.checkLeverage(c.UserID, c.Symbol, c.Leverage, price); err != nil {
		return CommandResult{Err: err}
	}
	err := e.portfolio.SetLeverage(c.UserID, c.Symbol, c.Leverage)
	if err == nil && writer != nil {
		_ = writer.Record(events.EncodeLeverage(events.LeverageEvent{UserID: c.UserID, Symbol: c.Symbol, Leverage: c.Leverage}))
	}
	return CommandResult{Err: err}
}

type UpdateTpSlCmd struct {
	UserID     types.UserID
	Symbol     string
	TakeProfit types.Price
	StopLoss   types.Price
}

func (c *UpdateTpSlCmd) Apply(e *Engine, writer outbox.Writer) CommandResult {
	if c.UserID == 0 || c.Symbol == "" {
		return CommandResult{Err: errInvalidOrderRequest}
	}
	pos := e.portfolio.GetPosition(c.UserID, c.Symbol)
	if pos == nil || math.Sign(pos.Size) == 0 {
		return CommandResult{Err: constants.ErrNoPosition}
	}

	if pos.TPOrderID != 0 {
		if order, ok := e.store.GetUserOrder(c.UserID, pos.TPOrderID); ok {
			_ = e.store.Cancel(c.UserID, pos.TPOrderID)
			if writer != nil {
				_ = writer.Record(events.EncodeOrderCanceled(events.OrderCanceledEvent{UserID: c.UserID, OrderID: pos.TPOrderID, Timestamp: order.UpdatedAt}))
			}
			if e.publisher != nil {
				e.publisher.OnOrderUpdated(order)
			}
		}
		pos.TPOrderID = 0
		pos.TakeProfit = types.Price{}
	}
	if pos.SLOrderID != 0 {
		if order, ok := e.store.GetUserOrder(c.UserID, pos.SLOrderID); ok {
			_ = e.store.Cancel(c.UserID, pos.SLOrderID)
			if writer != nil {
				_ = writer.Record(events.EncodeOrderCanceled(events.OrderCanceledEvent{UserID: c.UserID, OrderID: pos.SLOrderID, Timestamp: order.UpdatedAt}))
			}
			if e.publisher != nil {
				e.publisher.OnOrderUpdated(order)
			}
		}
		pos.SLOrderID = 0
		pos.StopLoss = types.Price{}
	}

	if math.Sign(c.TakeProfit) > 0 {
		takeProfit := e.store.Create(
			c.UserID,
			c.Symbol,
			constants.CATEGORY_LINEAR,
			constants.ORDER_ORIGIN_USER,
			oppositeSideForPosition(pos.Size),
			constants.ORDER_TYPE_LIMIT,
			constants.TIF_GTC,
			c.TakeProfit,
			math.AbsFixed(pos.Size),
			c.TakeProfit,
			true,
			true,
			constants.STOP_ORDER_TYPE_TAKE_PROFIT,
			triggerDirectionForOrder(
				constants.STOP_ORDER_TYPE_TAKE_PROFIT,
				oppositeSideForPosition(pos.Size),
				c.TakeProfit,
			),
		)
		pos.TPOrderID = takeProfit.ID
		pos.TakeProfit = c.TakeProfit
		if writer != nil {
			_ = writer.Record(events.EncodeOrderPlaced(events.OrderPlacedEvent{
				Order:      takeProfit,
				Instrument: e.registry.GetInstrument(takeProfit.Symbol),
			}))
		}
		if e.publisher != nil {
			e.publisher.OnOrderUpdated(takeProfit)
		}
	}

	if math.Sign(c.StopLoss) > 0 {
		stopLoss := e.store.Create(
			c.UserID,
			c.Symbol,
			constants.CATEGORY_LINEAR,
			constants.ORDER_ORIGIN_USER,
			oppositeSideForPosition(pos.Size),
			constants.ORDER_TYPE_LIMIT,
			constants.TIF_GTC,
			c.StopLoss,
			math.AbsFixed(pos.Size),
			c.StopLoss,
			true,
			true,
			constants.STOP_ORDER_TYPE_STOP_LOSS,
			triggerDirectionForOrder(
				constants.STOP_ORDER_TYPE_STOP_LOSS,
				oppositeSideForPosition(pos.Size),
				c.StopLoss,
			),
		)
		pos.SLOrderID = stopLoss.ID
		pos.StopLoss = c.StopLoss
		if writer != nil {
			_ = writer.Record(events.EncodeOrderPlaced(events.OrderPlacedEvent{
				Order:      stopLoss,
				Instrument: e.registry.GetInstrument(stopLoss.Symbol),
			}))
		}
		if e.publisher != nil {
			e.publisher.OnOrderUpdated(stopLoss)
		}
	}

	return CommandResult{}
}

type PublicTradesCmd struct {
	Category int8
	Symbol   string
}

func (c *PublicTradesCmd) Apply(e *Engine, _ outbox.Writer) CommandResult {
	return CommandResult{Trades: e.tradeFeed.Recent(c.Category, c.Symbol)}
}

type CreateDepositCmd struct {
	UserID      types.UserID
	Asset       string
	Amount      types.Quantity
	Destination string
	CreatedBy   types.FundingCreatedBy
	Message     string
}

func (c *CreateDepositCmd) Apply(e *Engine, writer outbox.Writer) CommandResult {
	request, err := e.portfolio.CreateDeposit(c.UserID, c.Asset, c.Amount, c.Destination, c.CreatedBy, c.Message)
	if err != nil {
		return CommandResult{Err: err}
	}
	if writer != nil {
		_ = writer.Record(events.EncodeFundingCreated(*request))
	}
	return CommandResult{Funding: request}
}

type CreateWithdrawalCmd struct {
	UserID      types.UserID
	Asset       string
	Amount      types.Quantity
	Destination string
	CreatedBy   types.FundingCreatedBy
	Message     string
}

func (c *CreateWithdrawalCmd) Apply(e *Engine, writer outbox.Writer) CommandResult {
	request, err := e.portfolio.CreateWithdrawal(c.UserID, c.Asset, c.Amount, c.Destination, c.CreatedBy, c.Message)
	if err != nil {
		return CommandResult{Err: err}
	}
	if writer != nil {
		_ = writer.Record(events.EncodeFundingCreated(*request))
	}
	return CommandResult{Funding: request}
}

type ApproveFundingCmd struct {
	FundingID types.FundingID
}

func (c *ApproveFundingCmd) Apply(e *Engine, writer outbox.Writer) CommandResult {
	if c.FundingID == 0 {
		return CommandResult{Err: errInvalidFundingID}
	}
	request, err := e.portfolio.ApproveFunding(c.FundingID)
	if err != nil {
		return CommandResult{Err: err}
	}
	if writer != nil {
		_ = writer.Record(events.EncodeFundingStatus(events.FundingApproved, request.ID))
	}
	return CommandResult{Funding: request}
}

type RejectFundingCmd struct {
	FundingID types.FundingID
}

func (c *RejectFundingCmd) Apply(e *Engine, writer outbox.Writer) CommandResult {
	if c.FundingID == 0 {
		return CommandResult{Err: errInvalidFundingID}
	}
	request, err := e.portfolio.RejectFunding(c.FundingID)
	if err != nil {
		return CommandResult{Err: err}
	}
	if writer != nil {
		_ = writer.Record(events.EncodeFundingStatus(events.FundingRejected, request.ID))
	}
	return CommandResult{Funding: request}
}
