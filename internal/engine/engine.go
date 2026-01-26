package engine

import (
	"errors"
	"sync"

	"github.com/maxonlinux/meta-terminal-go/internal/clearing"
	"github.com/maxonlinux/meta-terminal-go/internal/oms"
	orderbook "github.com/maxonlinux/meta-terminal-go/internal/orderbook"
	"github.com/maxonlinux/meta-terminal-go/internal/portfolio"
	"github.com/maxonlinux/meta-terminal-go/internal/registry"
	"github.com/maxonlinux/meta-terminal-go/internal/trades"
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/outbox"
	"github.com/maxonlinux/meta-terminal-go/pkg/persistence"
	"github.com/maxonlinux/meta-terminal-go/pkg/snowflake"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/maxonlinux/meta-terminal-go/pkg/utils"
	"github.com/robaho/fixed"
)

type OrderCallback interface {
	OnChildOrderCreated(order *types.Order)
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
	errInvalidCommand      = errors.New("invalid engine command")
	errInvalidOrderRequest = errors.New("invalid order request")
	errInvalidFundingID    = errors.New("invalid funding id")
	errOrderNotFound       = errors.New("order not found")
)

type Engine struct {
	store      *oms.Service
	persist    *persistence.Store
	outbox     *outbox.Outbox
	books      map[int8]map[string]*orderbook.OrderBook // category -> symbol -> orderbook
	clearing   *clearing.Service
	portfolio  *portfolio.Service
	tradeFeed  *trades.TradeFeed
	lastPrices map[string]types.Price // symbol -> price
	registry   *registry.Registry
	callback   OrderCallback
	locksMu    sync.Mutex
	locks      map[bookLockKey]*sync.Mutex // symbol & category -> mutex
}

func NewEngine(persist *persistence.Store, ob *outbox.Outbox, reg *registry.Registry, cb OrderCallback) *Engine {
	store := oms.NewService()
	e := &Engine{
		store:      store,
		persist:    persist,
		outbox:     ob,
		books:      make(map[int8]map[string]*orderbook.OrderBook),
		tradeFeed:  trades.NewTradeFeed(),
		lastPrices: make(map[string]types.Price),
		registry:   reg,
		callback:   cb,
		locks:      make(map[bookLockKey]*sync.Mutex),
	}
	portfolioService := portfolio.New(func(userID types.UserID, symbol string, size types.Quantity) {
		e.onPositionReduce(userID, symbol, size)
	}, reg)
	clearingService := clearing.New(portfolioService, reg)

	e.clearing = clearingService
	e.portfolio = portfolioService

	for _, cat := range []int8{constants.CATEGORY_SPOT, constants.CATEGORY_LINEAR} {
		e.books[cat] = make(map[string]*orderbook.OrderBook)
	}

	e.restoreState()

	return e
}

func (e *Engine) Shutdown() {
}

func (e *Engine) Registry() *registry.Registry {
	return e.registry
}

func (e *Engine) Portfolio() *portfolio.Service {
	return e.portfolio
}

func (e *Engine) Store() *oms.Service {
	return e.store
}

func (e *Engine) TradeFeed() *trades.TradeFeed {
	return e.tradeFeed
}

func (e *Engine) ReadBook(category int8, symbol string) *orderbook.OrderBook {
	bookSet, ok := e.books[category]
	if !ok {
		return nil
	}
	return bookSet[symbol]
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
		if order, ok := e.store.Get(c.OrderID); ok {
			result = apply(order.Symbol, order.Category)
		} else {
			result = CommandResult{Err: errOrderNotFound}
		}
	case *AmendOrderCmd:
		if order, ok := e.store.Get(c.OrderID); ok {
			result = apply(order.Symbol, order.Category)
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

func (e *Engine) onPositionReduce(userID types.UserID, symbol string, size types.Quantity) {
	e.store.OnPositionReduce(userID, symbol, size)
}

func (e *Engine) OnPriceTick(symbol string, price types.Price) {
	e.lastPrices[symbol] = price
	e.store.OnPriceTick(symbol, price, func(order *types.Order) {
		if e.callback != nil {
			e.callback.OnChildOrderCreated(order)
		}
	})
}

type bookLockKey struct {
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

func (e *Engine) restoreState() {
	if e.persist == nil {
		return
	}
	_ = e.persist.LoadBalances(func(balance *types.Balance) bool {
		e.portfolio.LoadBalance(balance)
		return true
	})
	_ = e.persist.LoadPositions(func(pos *types.Position) bool {
		e.portfolio.LoadPosition(pos)
		return true
	})
	_ = e.persist.LoadOrders(func(order *types.Order) bool {
		e.store.Load(order)
		if order == nil {
			return true
		}
		if order.IsConditional {
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
		if math.Cmp(req.Price, inst.MinPrice) < 0 || math.Cmp(req.Price, inst.MaxPrice) > 0 {
			return CommandResult{Err: constants.ErrPriceOutOfBounds}
		}
		if math.Cmp(req.Quantity, inst.MinQty) < 0 || math.Cmp(req.Quantity, inst.MaxQty) > 0 {
			return CommandResult{Err: constants.ErrQtyOutOfBounds}
		}
		req.Price = types.Price(math.RoundTo(req.Price, inst.TickSize))
		req.Quantity = types.Quantity(math.RoundTo(req.Quantity, inst.LotSize))
	} else if e.registry != nil {
		return CommandResult{Err: constants.ErrInstrumentNotFound}
	}

	if !req.TriggerPrice.IsZero() {
		if req.Category == constants.CATEGORY_SPOT {
			return CommandResult{Err: constants.ErrConditionalSpot}
		}

		switch req.Side {
		case constants.ORDER_SIDE_BUY:
			if math.Cmp(req.TriggerPrice, req.Price) >= 0 {
				return CommandResult{Err: constants.ErrInvalidTriggerForBuy}
			}
		case constants.ORDER_SIDE_SELL:
			if math.Cmp(req.TriggerPrice, req.Price) <= 0 {
				return CommandResult{Err: constants.ErrInvalidTriggerForSell}
			}
		}
	}

	if req.ReduceOnly && req.Category == constants.CATEGORY_SPOT {
		return CommandResult{Err: constants.ErrReduceOnlySpot}
	}

	book, err := e.getBook(req.Category, req.Symbol)
	if err != nil {
		return CommandResult{Err: err}
	}

	if req.TIF == constants.TIF_POST_ONLY {
		if req.Type == constants.ORDER_TYPE_MARKET || book.WouldCross(req.Side, req.Price) {
			return CommandResult{Err: constants.ErrPostOnlyWouldMatch}
		}
	}

	order := e.store.Build(
		req.UserID,
		req.Symbol,
		req.Category,
		req.Side,
		req.Type,
		req.TIF,
		req.Price,
		req.Quantity,
		req.TriggerPrice,
		req.ReduceOnly,
		req.CloseOnTrigger,
		req.StopOrderType,
	)

	if order.IsConditional {
		e.store.Add(order, writer)
		return CommandResult{Order: order}
	}

	limitPrice := order.Price
	if order.Type == constants.ORDER_TYPE_MARKET {
		limitPrice = types.Price{}
	}

	var buf [8]types.Match
	matches := book.GetMatches(order, limitPrice, buf[:0])
	var matchQty types.Quantity
	var matchNotional types.Quantity
	for i := range matches {
		matchQty = math.Add(matchQty, matches[i].Quantity)
		matchNotional = math.Add(matchNotional, math.Mul(matches[i].Price, matches[i].Quantity))
	}
	remaining := math.Sub(order.Quantity, matchQty)

	if req.TIF == constants.TIF_FOK && math.Sign(remaining) > 0 {
		return CommandResult{Err: constants.ErrFOKInsufficientLiquidity}
	}

	if math.Sign(matchQty) > 0 {
		avgPrice := types.Price(math.Div(matchNotional, matchQty))
		if err := e.clearing.Reserve(order.UserID, order.Symbol, order.Category, order.Side, matchQty, avgPrice, writer); err != nil {
			return CommandResult{Err: err}
		}
	}

	if math.Sign(remaining) > 0 && (req.TIF == constants.TIF_POST_ONLY || req.TIF == constants.TIF_GTC) {
		if err := e.clearing.Reserve(order.UserID, order.Symbol, order.Category, order.Side, remaining, order.Price, writer); err != nil {
			return CommandResult{Err: err}
		}
	}

	e.store.Add(order, writer)

	for i := range matches {
		match := matches[i]
		match.ID = types.TradeID(snowflake.Next())
		match.Timestamp = utils.NowNano()
		e.applyTrade(book, match, writer)
	}

	if req.TIF == constants.TIF_IOC || order.Type == constants.ORDER_TYPE_MARKET {
		if math.Cmp(order.Filled, order.Quantity) != 0 {
			_ = e.store.Cancel(order.ID, writer)
			return CommandResult{Order: order}
		}
		return CommandResult{Order: order}
	}

	if req.TIF == constants.TIF_POST_ONLY {
		book.Add(order)
		return CommandResult{Order: order}
	}

	if math.Sign(remaining) > 0 {
		book.Add(order)
	}
	return CommandResult{Order: order}
}

type CancelOrderCmd struct {
	OrderID types.OrderID
}

func (c *CancelOrderCmd) Apply(e *Engine, writer outbox.Writer) CommandResult {
	if c.OrderID == 0 {
		return CommandResult{Err: errInvalidOrderRequest}
	}

	if err := e.store.Cancel(c.OrderID, writer); err != nil {
		return CommandResult{Err: err}
	}
	if order, ok := e.store.Get(c.OrderID); ok {
		remaining := math.Sub(order.Quantity, order.Filled)
		if math.Sign(remaining) > 0 {
			e.clearing.Release(order.UserID, order.Symbol, order.Category, order.Side, remaining, order.Price, writer)
		}
		if book, err := e.getBook(order.Category, order.Symbol); err == nil {
			_ = book.Remove(order.ID)
		}
		return CommandResult{Order: order}
	}
	return CommandResult{}
}

type AmendOrderCmd struct {
	OrderID types.OrderID
	NewQty  types.Quantity
}

func (c *AmendOrderCmd) Apply(e *Engine, writer outbox.Writer) CommandResult {
	if c.OrderID == 0 {
		return CommandResult{Err: errInvalidOrderRequest}
	}

	if err := e.store.Amend(c.OrderID, c.NewQty, writer); err != nil {
		return CommandResult{Err: err}
	}
	order, ok := e.store.Get(c.OrderID)
	if ok {
		return CommandResult{Order: order}
	}
	return CommandResult{}
}

type SetLeverageCmd struct {
	UserID   types.UserID
	Symbol   string
	Leverage types.Leverage
}

func (c *SetLeverageCmd) Apply(e *Engine, writer outbox.Writer) CommandResult {
	price := e.lastPrices[c.Symbol]
	if price.Sign() <= 0 {
		return CommandResult{Err: constants.ErrPriceUnavailable}
	}
	if err := e.checkLeverage(c.UserID, c.Symbol, c.Leverage, price); err != nil {
		return CommandResult{Err: err}
	}
	return CommandResult{Err: e.portfolio.SetLeverage(c.UserID, c.Symbol, c.Leverage, writer)}
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
	request, err := e.portfolio.CreateDeposit(c.UserID, c.Asset, c.Amount, c.Destination, c.CreatedBy, c.Message, writer)
	if err != nil {
		return CommandResult{Err: err}
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
	request, err := e.portfolio.CreateWithdrawal(c.UserID, c.Asset, c.Amount, c.Destination, c.CreatedBy, c.Message, writer)
	if err != nil {
		return CommandResult{Err: err}
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
	request, err := e.portfolio.ApproveFunding(c.FundingID, writer)
	if err != nil {
		return CommandResult{Err: err}
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
	request, err := e.portfolio.RejectFunding(c.FundingID, writer)
	if err != nil {
		return CommandResult{Err: err}
	}
	return CommandResult{Funding: request}
}
