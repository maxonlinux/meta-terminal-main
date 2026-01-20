package engine

import (
	"errors"

	"github.com/maxonlinux/meta-terminal-go/internal/clearing"
	"github.com/maxonlinux/meta-terminal-go/internal/matching"
	"github.com/maxonlinux/meta-terminal-go/internal/oms"
	orderbook "github.com/maxonlinux/meta-terminal-go/internal/orderbook"
	"github.com/maxonlinux/meta-terminal-go/internal/portfolio"
	"github.com/maxonlinux/meta-terminal-go/internal/registry"
	"github.com/maxonlinux/meta-terminal-go/internal/trades"
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/robaho/fixed"
)

type OrderCallback interface {
	OnChildOrderCreated(order *types.Order)
}

type Command interface {
	Apply(*Engine) CommandResult
}

type CommandResult struct {
	Err     error
	Trades  []types.Trade
	Funding *types.FundingRequest
	Order   *types.Order
}

type queuedCommand struct {
	cmd   Command
	reply chan CommandResult
}

var (
	errInvalidCommand      = errors.New("invalid engine command")
	errInvalidOrderRequest = errors.New("invalid order request")
	errInvalidFundingID    = errors.New("invalid funding id")
)

type Engine struct {
	store      *oms.Service
	books      map[int8]map[string]*orderbook.OrderBook
	clearing   *clearing.Service
	portfolio  *portfolio.Service
	tradeFeed  *trades.TradeFeed
	lastPrices map[string]types.Price
	registry   *registry.Registry
	commands   chan queuedCommand
	callback   OrderCallback
	done       chan struct{}
}

func NewEngine(store *oms.Service, reg *registry.Registry, cb OrderCallback) *Engine {
	var engineRef *Engine
	portfolioService := portfolio.New(nil, func(userID types.UserID, symbol string, size types.Quantity) {
		if engineRef != nil {
			engineRef.onPositionReduce(userID, symbol, size)
		}
	}, reg)
	clearingService := clearing.New(portfolioService, reg)

	e := &Engine{
		store:      store,
		books:      make(map[int8]map[string]*orderbook.OrderBook),
		clearing:   clearingService,
		portfolio:  portfolioService,
		tradeFeed:  trades.NewTradeFeed(),
		lastPrices: make(map[string]types.Price),
		registry:   reg,
		commands:   make(chan queuedCommand, constants.ENGINE_COMMAND_QUEUE_SIZE),
		callback:   cb,
		done:       make(chan struct{}),
	}

	for _, cat := range []int8{constants.CATEGORY_SPOT, constants.CATEGORY_LINEAR} {
		e.books[cat] = make(map[string]*orderbook.OrderBook)
	}

	go func() {
		for {
			select {
			case item := <-e.commands:
				result := item.cmd.Apply(e)
				if item.reply != nil {
					item.reply <- result
				}
			case <-e.done:
				return
			}
		}
	}()

	engineRef = e
	return e
}

func (e *Engine) Shutdown() {
	select {
	case <-e.done:
		return
	default:
		close(e.done)
	}
}

func (e *Engine) Cmd(cmd Command) CommandResult {
	reply := make(chan CommandResult, 1)
	e.commands <- queuedCommand{cmd: cmd, reply: reply}
	return <-reply
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

type PlaceOrderCmd struct {
	Req *types.PlaceOrderRequest
}

func (c *PlaceOrderCmd) Apply(e *Engine) CommandResult {
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

	if err := e.clearing.Reserve(req.UserID, req.Symbol, req.Category, req.Side, req.Quantity, req.Price); err != nil {
		return CommandResult{Err: err}
	}

	order := e.store.Create(
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

	book, err := e.getBook(order.Category, order.Symbol)
	if err != nil {
		return CommandResult{Err: err}
	}

	matchErr := matching.MatchOrder(order, book, e.applyMatch)
	if matchErr != nil {
		_ = e.store.Cancel(order.ID)
		remaining := math.Sub(order.Quantity, order.Filled)
		if math.Sign(remaining) > 0 {
			e.clearing.Release(order.UserID, order.Symbol, order.Category, order.Side, remaining, order.Price)
		}
		return CommandResult{Err: matchErr}
	}

	if order.Status == constants.ORDER_STATUS_CANCELED || order.Status == constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED {
		remaining := math.Sub(order.Quantity, order.Filled)
		if math.Sign(remaining) > 0 {
			e.clearing.Release(order.UserID, order.Symbol, order.Category, order.Side, remaining, order.Price)
		}
	}
	return CommandResult{Order: order}
}

type CancelOrderCmd struct {
	OrderID types.OrderID
}

func (c *CancelOrderCmd) Apply(e *Engine) CommandResult {
	if c.OrderID == 0 {
		return CommandResult{Err: errInvalidOrderRequest}
	}

	if err := e.store.Cancel(c.OrderID); err != nil {
		return CommandResult{Err: err}
	}
	if order, ok := e.store.Get(c.OrderID); ok {
		remaining := math.Sub(order.Quantity, order.Filled)
		if math.Sign(remaining) > 0 {
			e.clearing.Release(order.UserID, order.Symbol, order.Category, order.Side, remaining, order.Price)
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

func (c *AmendOrderCmd) Apply(e *Engine) CommandResult {
	if c.OrderID == 0 {
		return CommandResult{Err: errInvalidOrderRequest}
	}

	if err := e.store.Amend(c.OrderID, c.NewQty); err != nil {
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

func (c *SetLeverageCmd) Apply(e *Engine) CommandResult {
	price := e.lastPrices[c.Symbol]
	if price.Sign() <= 0 {
		return CommandResult{Err: constants.ErrPriceUnavailable}
	}
	if err := e.checkLeverage(c.UserID, c.Symbol, c.Leverage, price); err != nil {
		return CommandResult{Err: err}
	}
	return CommandResult{Err: e.portfolio.SetLeverage(c.UserID, c.Symbol, c.Leverage)}
}

type PublicTradesCmd struct {
	Category int8
	Symbol   string
}

func (c *PublicTradesCmd) Apply(e *Engine) CommandResult {
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

func (c *CreateDepositCmd) Apply(e *Engine) CommandResult {
	request, err := e.portfolio.CreateDeposit(c.UserID, c.Asset, c.Amount, c.Destination, c.CreatedBy, c.Message)
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

func (c *CreateWithdrawalCmd) Apply(e *Engine) CommandResult {
	request, err := e.portfolio.CreateWithdrawal(c.UserID, c.Asset, c.Amount, c.Destination, c.CreatedBy, c.Message)
	if err != nil {
		return CommandResult{Err: err}
	}
	return CommandResult{Funding: request}
}

type ApproveFundingCmd struct {
	FundingID types.FundingID
}

func (c *ApproveFundingCmd) Apply(e *Engine) CommandResult {
	if c.FundingID == 0 {
		return CommandResult{Err: errInvalidFundingID}
	}
	request, err := e.portfolio.ApproveFunding(c.FundingID)
	if err != nil {
		return CommandResult{Err: err}
	}
	return CommandResult{Funding: request}
}

type RejectFundingCmd struct {
	FundingID types.FundingID
}

func (c *RejectFundingCmd) Apply(e *Engine) CommandResult {
	if c.FundingID == 0 {
		return CommandResult{Err: errInvalidFundingID}
	}
	request, err := e.portfolio.RejectFunding(c.FundingID)
	if err != nil {
		return CommandResult{Err: err}
	}
	return CommandResult{Funding: request}
}

func (e *Engine) GetOrders(userID types.UserID, symbol string, category int8) []*types.Order {
	var result []*types.Order
	e.store.Iterate(func(o *types.Order) bool {
		if o.UserID == userID {
			if symbol == "" || o.Symbol == symbol {
				if category == 0 || o.Category == category {
					result = append(result, o)
				}
			}
		}
		return true
	})
	return result
}

func (e *Engine) GetOrder(id types.OrderID) (*types.Order, bool) {
	return e.store.Get(id)
}

func (e *Engine) GetPositions(userID types.UserID) []*types.Position {
	var result []*types.Position
	positions := e.portfolio.GetPositions(userID)
	for _, pos := range positions {
		result = append(result, pos)
	}
	return result
}

func (e *Engine) GetBalances(userID types.UserID) []*types.Balance {
	var result []*types.Balance
	balances := e.portfolio.GetBalances(userID)
	for _, bal := range balances {
		result = append(result, bal)
	}
	return result
}

func (e *Engine) GetInstruments(symbol string) []*types.Instrument {
	if symbol != "" {
		inst := e.registry.GetInstrument(symbol)
		if inst != nil {
			return []*types.Instrument{inst}
		}
		return nil
	}
	return e.registry.GetInstruments()
}

func (e *Engine) GetOrderBook(symbol string) *OrderBookResponse {
	return e.SnapshotOrderBook(symbol)
}

func (e *Engine) GetPublicTrades(symbol string) []types.Trade {
	return e.tradeFeed.Recent(constants.CATEGORY_SPOT, symbol)
}

type OrderBookResponse struct {
	Symbol string      `json:"symbol"`
	Bids   []BookLevel `json:"bids"`
	Asks   []BookLevel `json:"asks"`
}

type BookLevel struct {
	Price string `json:"price"`
	Total string `json:"total"`
}

func (e *Engine) SnapshotOrderBook(symbol string) *OrderBookResponse {
	_, err := e.getBook(constants.CATEGORY_SPOT, symbol)
	if err != nil {
		return nil
	}
	// Simplified snapshot - real implementation would need proper locking
	return &OrderBookResponse{
		Symbol: symbol,
		Bids:   []BookLevel{},
		Asks:   []BookLevel{},
	}
}
