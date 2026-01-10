package oms

import (
	"context"
	"errors"
	"sort"
	"sync"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/messaging"
	"github.com/anomalyco/meta-terminal-go/internal/orderbook"
	"github.com/anomalyco/meta-terminal-go/internal/pool"
	"github.com/anomalyco/meta-terminal-go/internal/triggers"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

type Portfolio interface {
	GetPositions(userID types.UserID) []*types.Position
	GetPosition(userID types.UserID, symbol string) *types.Position
	GetBalance(userID types.UserID, asset string) *types.UserBalance
}

type Clearing interface {
	Reserve(userID types.UserID, symbol string, category int8, side int8, qty types.Quantity, price types.Price) error
	Release(userID types.UserID, symbol string, category int8, side int8, qty types.Quantity, price types.Price)
	ExecuteTrade(trade *types.Trade, taker *types.Order, maker *types.Order)
}

type Config struct {
	NATSURL      string
	StreamPrefix string
}

type Service struct {
	nats       *messaging.NATS
	orderbooks map[int8]map[string]*orderbook.OrderBook
	orders     map[types.UserID]map[types.OrderID]*types.Order
	triggerMon *triggers.Monitor
	portfolio  Portfolio
	clearing   Clearing

	reduceOnlyCommitment map[types.UserID]map[string]int64
	lastPrices           map[string]types.Price

	mu sync.RWMutex
}

func New(cfg Config, portfolio Portfolio, clearing Clearing) (*Service, error) {
	n, err := messaging.New(messaging.Config{URL: cfg.NATSURL, StreamPrefix: cfg.StreamPrefix})
	if err != nil {
		return nil, err
	}

	return &Service{
		nats: n,
		orderbooks: map[int8]map[string]*orderbook.OrderBook{
			constants.CATEGORY_SPOT:   make(map[string]*orderbook.OrderBook),
			constants.CATEGORY_LINEAR: make(map[string]*orderbook.OrderBook),
		},
		orders:               make(map[types.UserID]map[types.OrderID]*types.Order),
		triggerMon:           triggers.New(),
		portfolio:            portfolio,
		clearing:             clearing,
		reduceOnlyCommitment: make(map[types.UserID]map[string]int64),
		lastPrices:           make(map[string]types.Price),
	}, nil
}

func (s *Service) Start(ctx context.Context) error {
	s.nats.Subscribe(ctx, messaging.OrderPlaceTopic(""), "oms-place", s.handleOrderPlace)
	s.nats.Subscribe(ctx, messaging.OrderEventTopic(""), "oms-events", s.handleOrderEvent)
	s.nats.Subscribe(ctx, messaging.PriceTickTopic(""), "oms-price", s.handlePriceTick)
	s.nats.Subscribe(ctx, messaging.PositionsEventTopic(""), "oms-positions", s.handlePositionUpdate)
	return nil
}

func (s *Service) handleOrderEvent(data []byte) {
}

func (s *Service) handleOrderPlace(data []byte) {
	var input types.OrderInput
	if err := messaging.DecodeGob(data, &input); err != nil {
		return
	}
	s.PlaceOrder(context.Background(), &input)
}

func (s *Service) handlePriceTick(data []byte) {
	var tick struct {
		Symbol string
		Price  types.Price
	}
	if err := messaging.DecodeGob(data, &tick); err != nil {
		return
	}

	s.mu.Lock()
	s.lastPrices[tick.Symbol] = tick.Price
	s.mu.Unlock()

	s.OnPriceTick(tick.Symbol, tick.Price)
}

func (s *Service) handlePositionUpdate(data []byte) {
	var update struct {
		UserID  types.UserID
		Symbol  string
		NewSize int64
		NewSide int8
	}
	if err := messaging.DecodeGob(data, &update); err != nil {
		return
	}
	s.OnPositionUpdate(update.UserID, update.Symbol, update.NewSize, update.NewSide)
}

func (s *Service) PlaceOrder(ctx context.Context, input *types.OrderInput) (*types.OrderResult, error) {
	if err := s.validateOrder(input); err != nil {
		return nil, err
	}

	if input.TriggerPrice > 0 {
		input.IsConditional = true
	}

	// Self-match prevention: check if order would match with own orders
	if err := s.checkSelfMatch(input); err != nil {
		return nil, err
	}

	if input.OCO != nil {
		return s.placeOCOOrder(ctx, input)
	}

	order := pool.GetOrder()
	order.ID = types.OrderID(poolGetOrderID())
	order.UserID = input.UserID
	order.Symbol = input.Symbol
	order.Category = input.Category
	order.Side = input.Side
	order.Type = input.Type
	order.TIF = input.TIF
	order.Status = constants.ORDER_STATUS_NEW
	order.Price = input.Price
	order.Quantity = input.Quantity
	order.Filled = 0
	order.CreatedAt = types.NowNano()
	order.UpdatedAt = order.CreatedAt
	order.TriggerPrice = input.TriggerPrice
	order.CloseOnTrigger = input.CloseOnTrigger
	order.ReduceOnly = input.ReduceOnly
	order.IsConditional = input.IsConditional

	if input.TriggerPrice > 0 {
		return s.placeConditionalOrder(ctx, order, input)
	}

	return s.placeNormalOrder(ctx, order, input)
}

func (s *Service) placeOCOOrder(ctx context.Context, input *types.OrderInput) (*types.OrderResult, error) {
	oco := input.OCO

	if input.Category == constants.CATEGORY_SPOT {
		return nil, ErrOCOSpot
	}

	positions := s.portfolio.GetPositions(input.UserID)
	var posSize int64
	var posSide int8
	for _, p := range positions {
		if p.Symbol == input.Symbol {
			posSize = p.Size
			posSide = p.Side
			break
		}
	}
	if posSize == 0 {
		return nil, ErrOCONoPosition
	}

	quantity := oco.Quantity
	isReduceOnly := quantity == 0

	if isReduceOnly {
		quantity = types.Quantity(posSize)
	}

	if int64(quantity) > posSize {
		quantity = types.Quantity(posSize)
	}

	var tpSide, slSide int8
	if posSide == constants.SIDE_LONG {
		tpSide = constants.ORDER_SIDE_SELL
		slSide = constants.ORDER_SIDE_SELL

		// Для LONG: TP trigger must be > SL trigger
		if oco.TakeProfit.TriggerPrice > 0 && oco.StopLoss.TriggerPrice > 0 {
			if oco.TakeProfit.TriggerPrice <= oco.StopLoss.TriggerPrice {
				return nil, ErrOCOTPTriggerInvalid
			}
		}
	} else if posSide == constants.SIDE_SHORT {
		tpSide = constants.ORDER_SIDE_BUY
		slSide = constants.ORDER_SIDE_BUY

		// Для SHORT: TP trigger must be < SL trigger
		if oco.TakeProfit.TriggerPrice > 0 && oco.StopLoss.TriggerPrice > 0 {
			if oco.TakeProfit.TriggerPrice >= oco.StopLoss.TriggerPrice {
				return nil, ErrOCOSLTriggerInvalid
			}
		}
	} else {
		return nil, ErrOCONoPosition
	}

	tpReduceOnly := oco.TakeProfit.ReduceOnly || isReduceOnly
	slReduceOnly := oco.StopLoss.ReduceOnly || isReduceOnly

	// Quantity=0 для OCO означает "закрыть всю позицию на момент триггера"
	// Поэтому устанавливаем Quantity=0 в ордерах, а реальное значение
	// будет взято из позиции в handleConditional/handleCloseOnTrigger
	tpInput := &types.OrderInput{
		UserID:         input.UserID,
		Symbol:         input.Symbol,
		Category:       input.Category,
		Side:           tpSide,
		Type:           constants.ORDER_TYPE_LIMIT,
		TIF:            constants.TIF_GTC,
		Quantity:       0, // Использовать размер позиции на момент триггера
		Price:          oco.TakeProfit.Price,
		TriggerPrice:   oco.TakeProfit.TriggerPrice,
		ReduceOnly:     tpReduceOnly,
		CloseOnTrigger: true, // OCO ордера всегда closeOnTrigger
	}

	slInput := &types.OrderInput{
		UserID:         input.UserID,
		Symbol:         input.Symbol,
		Category:       input.Category,
		Side:           slSide,
		Type:           constants.ORDER_TYPE_LIMIT,
		TIF:            constants.TIF_GTC,
		Quantity:       0, // Использовать размер позиции на момент триггера
		Price:          oco.StopLoss.Price,
		TriggerPrice:   oco.StopLoss.TriggerPrice,
		ReduceOnly:     slReduceOnly,
		CloseOnTrigger: true, // OCO ордера всегда closeOnTrigger
	}

	tpOrder, slOrder, tpResult, err := s.placeOCOInternal(ctx, tpInput, slInput, quantity)
	if err != nil {
		return nil, err
	}

	if tpReduceOnly {
		s.addReduceOnlyCommitment(tpOrder.UserID, tpOrder.Symbol, int64(tpOrder.Quantity))
	}
	if slReduceOnly {
		s.addReduceOnlyCommitment(slOrder.UserID, slOrder.Symbol, int64(slOrder.Quantity))
	}

	s.publishOrderEvent(tpOrder)
	s.publishOrderEvent(slOrder)

	return tpResult, nil
}

func (s *Service) addReduceOnlyCommitment(userID types.UserID, symbol string, amount int64) {
	if s.reduceOnlyCommitment[userID] == nil {
		s.reduceOnlyCommitment[userID] = make(map[string]int64)
	}
	s.reduceOnlyCommitment[userID][symbol] += amount
}

func (s *Service) placeOCOInternal(ctx context.Context, tpInput, slInput *types.OrderInput, ocoQuantity types.Quantity) (*types.Order, *types.Order, *types.OrderResult, error) {
	groupID := int64(poolGetOrderID())

	tpOrder := pool.GetOrder()
	tpOrder.ID = types.OrderID(poolGetOrderID())
	tpOrder.UserID = tpInput.UserID
	tpOrder.Symbol = tpInput.Symbol
	tpOrder.Category = tpInput.Category
	tpOrder.Side = tpInput.Side
	tpOrder.Type = tpInput.Type
	tpOrder.TIF = tpInput.TIF
	tpOrder.Status = constants.ORDER_STATUS_UNTRIGGERED
	tpOrder.Price = tpInput.Price
	tpOrder.Quantity = tpInput.Quantity
	tpOrder.Filled = 0
	tpOrder.CreatedAt = types.NowNano()
	tpOrder.UpdatedAt = tpOrder.CreatedAt
	tpOrder.TriggerPrice = tpInput.TriggerPrice
	tpOrder.ReduceOnly = tpInput.ReduceOnly
	tpOrder.CloseOnTrigger = true
	tpOrder.StopOrderType = constants.STOP_ORDER_TYPE_TAKE_PROFIT
	tpOrder.OrderLinkId = groupID

	s.mu.Lock()
	if s.orders[tpOrder.UserID] == nil {
		s.orders[tpOrder.UserID] = make(map[types.OrderID]*types.Order)
	}
	s.orders[tpOrder.UserID][tpOrder.ID] = tpOrder
	s.mu.Unlock()

	s.triggerMon.Add(tpOrder)

	slOrder := pool.GetOrder()
	slOrder.ID = types.OrderID(poolGetOrderID())
	slOrder.UserID = slInput.UserID
	slOrder.Symbol = slInput.Symbol
	slOrder.Category = slInput.Category
	slOrder.Side = slInput.Side
	slOrder.Type = slInput.Type
	slOrder.TIF = slInput.TIF
	slOrder.Status = constants.ORDER_STATUS_UNTRIGGERED
	slOrder.Price = slInput.Price
	slOrder.Quantity = slInput.Quantity
	slOrder.Filled = 0
	slOrder.CreatedAt = types.NowNano()
	slOrder.UpdatedAt = slOrder.CreatedAt
	slOrder.TriggerPrice = slInput.TriggerPrice
	slOrder.ReduceOnly = slInput.ReduceOnly
	slOrder.CloseOnTrigger = true
	slOrder.StopOrderType = constants.STOP_ORDER_TYPE_STOP_LOSS
	slOrder.OrderLinkId = groupID

	s.mu.Lock()
	s.orders[slOrder.UserID][slOrder.ID] = slOrder
	s.mu.Unlock()

	s.triggerMon.Add(slOrder)

	// Для OCO с quantity=0 возвращаем размер позиции в качестве remaining
	// (ордера используют 0 как "полное закрытие позиции")
	return tpOrder, slOrder, &types.OrderResult{
		Orders:    []*types.Order{tpOrder, slOrder},
		Trades:    nil,
		Filled:    0,
		Remaining: ocoQuantity,
		Status:    constants.ORDER_STATUS_UNTRIGGERED,
	}, nil
}

func (s *Service) validateOrder(input *types.OrderInput) error {
	// Устанавливаем IsConditional если есть triggerPrice
	if input.TriggerPrice > 0 {
		input.IsConditional = true
	}

	// === FIELD VALIDATION ===

	// Quantity must be > 0 for regular orders
	// Quantity can be 0 for conditional/closeOnTrigger orders (use position size at trigger)
	if input.Quantity < 0 {
		return ErrInvalidQuantity
	}
	if input.Quantity == 0 && input.TriggerPrice == 0 && !input.CloseOnTrigger {
		return ErrInvalidQuantity
	}

	// Price must be >= 0 for LIMIT orders
	if input.Type == constants.ORDER_TYPE_LIMIT && input.Price < 0 {
		return ErrInvalidPrice
	}

	// Symbol must not be empty (basic format check)
	if input.Symbol == "" {
		return ErrInvalidSymbol
	}

	// Validate symbol format (basic check for common quote assets)
	if !s.isValidSymbolFormat(input.Symbol) {
		return ErrInvalidSymbol
	}

	// Category must be valid
	if input.Category != constants.CATEGORY_SPOT && input.Category != constants.CATEGORY_LINEAR {
		return ErrInvalidCategory
	}

	// Side must be valid
	if input.Side != constants.ORDER_SIDE_BUY && input.Side != constants.ORDER_SIDE_SELL {
		return ErrInvalidSide
	}

	// Type must be valid
	if input.Type != constants.ORDER_TYPE_LIMIT && input.Type != constants.ORDER_TYPE_MARKET {
		return ErrInvalidOrderType
	}

	// TIF must be valid
	if input.TIF != constants.TIF_GTC && input.TIF != constants.TIF_IOC &&
		input.TIF != constants.TIF_FOK && input.TIF != constants.TIF_POST_ONLY {
		return ErrInvalidTIF
	}

	// StopOrderType must be valid (if provided)
	if input.StopOrderType < 0 || input.StopOrderType > constants.STOP_ORDER_TYPE_OCO {
		return ErrInvalidStopOrderType
	}

	if input.Category == constants.CATEGORY_SPOT {
		if input.ReduceOnly {
			return ErrReduceOnlySpot
		}
		if input.TriggerPrice > 0 {
			return ErrConditionalSpot
		}
		if input.CloseOnTrigger {
			return ErrCloseOnTriggerSpot
		}
	}

	if input.Category == constants.CATEGORY_LINEAR {
		if input.Type == constants.ORDER_TYPE_MARKET && input.TIF != constants.TIF_IOC && input.TIF != constants.TIF_FOK {
			return ErrMarketTIF
		}

		// CloseOnTrigger requires existing position
		if input.CloseOnTrigger {
			positions := s.portfolio.GetPositions(input.UserID)
			var posSize int64
			for _, p := range positions {
				if p.Symbol == input.Symbol {
					posSize = p.Size
					break
				}
			}
			if posSize == 0 {
				return ErrCloseOnTriggerNoPosition
			}
		}

		if input.ReduceOnly {
			positions := s.portfolio.GetPositions(input.UserID)
			var posSize int64
			for _, p := range positions {
				if p.Symbol == input.Symbol {
					posSize = p.Size
					break
				}
			}
			if posSize <= 0 {
				return ErrReduceOnlyNoPosition
			}

			commitment := s.reduceOnlyCommitment[input.UserID][input.Symbol]
			maxAllowed := posSize - commitment
			if maxAllowed <= 0 {
				return ErrReduceOnlyCommitmentExceeded
			}
			if int64(input.Quantity) > maxAllowed {
				input.Quantity = types.Quantity(maxAllowed)
			}
		}

		if input.TriggerPrice > 0 {
			currentPrice := s.GetLastPrice(input.Symbol)
			if currentPrice == 0 {
				return nil
			}
			if input.Side == constants.ORDER_SIDE_BUY && input.TriggerPrice >= currentPrice {
				return ErrInvalidTriggerPrice
			}
			if input.Side == constants.ORDER_SIDE_SELL && input.TriggerPrice <= currentPrice {
				return ErrInvalidTriggerPrice
			}
		}
	}

	return nil
}

// isValidSymbolFormat проверяет базовый формат символа (BTCUSDT, ETHUSDT, etc.)
func (s *Service) isValidSymbolFormat(symbol string) bool {
	if len(symbol) < 4 || len(symbol) > 20 {
		return false
	}

	// Проверяем, что символ заканчивается на известный quote asset
	quoteAssets := []string{"USDT", "USD", "USDC", "BUSD", "DAI"}
	for _, q := range quoteAssets {
		if len(symbol) > len(q) && symbol[len(symbol)-len(q):] == q {
			// Проверяем, что base часть не пустая и содержит только буквы
			base := symbol[:len(symbol)-len(q)]
			if len(base) < 2 {
				return false
			}
			for _, c := range base {
				if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')) {
					return false
				}
			}
			return true
		}
	}

	// Для других форматов просто проверяем что строка валидная
	for _, c := range symbol {
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')) {
			return false
		}
	}
	return len(symbol) >= 3
}

// checkSelfMatch предотвращает выставление ордера, который может исполниться сам с собой.
// Self-match происходит когда ордер исполняется против собственного ордера в orderbook.
func (s *Service) checkSelfMatch(input *types.OrderInput) error {
	// Conditional и closeOnTrigger ордера не попадают в orderbook
	if input.TriggerPrice > 0 || input.CloseOnTrigger {
		return nil
	}

	// Market ордера проверяем отдельно (у них нет конкретной цены)
	if input.Type == constants.ORDER_TYPE_MARKET {
		return s.checkSelfMatchMarket(input)
	}

	ob := s.getOrderBook(input.Category, input.Symbol)

	// Для BUY: проверяем есть ли SELL ордера этого пользователя по цене <= input.Price
	// Для SELL: проверяем есть ли BUY ордера этого пользователя по цене >= input.Price
	if input.Side == constants.ORDER_SIDE_BUY {
		// Проверяем худшую цену (best ask) — если у пользователя есть ордер на sell
		// по цене ниже или равной нашему buy price, будет self-match
		_, _, askPrices, askQtys := ob.Depth(1)
		if len(askPrices) > 0 && len(askQtys) > 0 && askQtys[0] > 0 {
			if s.userHasOrderAtPriceOrBetter(input.UserID, input.Symbol, constants.ORDER_SIDE_SELL, askPrices[0]) {
				return ErrSelfMatch
			}
		}
	} else {
		// SELL — проверяем best bid
		bidPrices, _, _, _ := ob.Depth(1)
		if len(bidPrices) > 0 && bidPrices[0] > 0 {
			if s.userHasOrderAtPriceOrBetter(input.UserID, input.Symbol, constants.ORDER_SIDE_BUY, bidPrices[0]) {
				return ErrSelfMatch
			}
		}
	}

	return nil
}

func (s *Service) checkSelfMatchMarket(input *types.OrderInput) error {
	ob := s.getOrderBook(input.Category, input.Symbol)
	_ = ob

	// Для MARKET ордера проверяем есть ли вообще ордера этого пользователя в orderbook
	if input.Side == constants.ORDER_SIDE_BUY {
		// Проверяем есть ли SELL ордера этого пользователя
		if s.userHasOrdersOnSide(input.UserID, input.Symbol, constants.ORDER_SIDE_SELL) {
			return ErrSelfMatch
		}
	} else {
		// SELL — проверяем есть ли BUY ордера этого пользователя
		if s.userHasOrdersOnSide(input.UserID, input.Symbol, constants.ORDER_SIDE_BUY) {
			return ErrSelfMatch
		}
	}

	return nil
}

func (s *Service) userHasOrderAtPriceOrBetter(userID types.UserID, symbol string, side int8, price types.Price) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	userOrders, ok := s.orders[userID]
	if !ok {
		return false
	}

	for _, order := range userOrders {
		if order.Symbol != symbol {
			continue
		}
		if order.Status != constants.ORDER_STATUS_NEW && order.Status != constants.ORDER_STATUS_PARTIALLY_FILLED {
			continue
		}
		if order.Type == constants.ORDER_TYPE_MARKET {
			return true // Market ордер исполнится по любой цене
		}
		if order.Side != side {
			continue
		}

		// Для BUY: проверяем price >= order.Price (наш ордер купит дороже или по той же цене)
		// Для SELL: проверяем price <= order.Price (наш ордер продаст дешевле или по той же цене)
		if side == constants.ORDER_SIDE_BUY {
			if price >= order.Price {
				return true
			}
		} else {
			if price <= order.Price {
				return true
			}
		}
	}

	return false
}

func (s *Service) userHasOrdersOnSide(userID types.UserID, symbol string, side int8) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	userOrders, ok := s.orders[userID]
	if !ok {
		return false
	}

	for _, order := range userOrders {
		if order.Symbol != symbol {
			continue
		}
		if order.Status != constants.ORDER_STATUS_NEW && order.Status != constants.ORDER_STATUS_PARTIALLY_FILLED {
			continue
		}
		if order.Side == side {
			return true
		}
	}

	return false
}

func (s *Service) placeConditionalOrder(ctx context.Context, order *types.Order, input *types.OrderInput) (*types.OrderResult, error) {
	order.Status = constants.ORDER_STATUS_UNTRIGGERED

	s.mu.Lock()
	if s.orders[order.UserID] == nil {
		s.orders[order.UserID] = make(map[types.OrderID]*types.Order)
	}
	s.orders[order.UserID][order.ID] = order
	s.mu.Unlock()

	s.triggerMon.Add(order)

	return &types.OrderResult{
		Orders:    []*types.Order{order},
		Trades:    nil,
		Filled:    0,
		Remaining: order.Quantity,
		Status:    constants.ORDER_STATUS_UNTRIGGERED,
	}, nil
}

func (s *Service) placeNormalOrder(ctx context.Context, order *types.Order, input *types.OrderInput) (*types.OrderResult, error) {
	s.mu.Lock()
	if s.orders[order.UserID] == nil {
		s.orders[order.UserID] = make(map[types.OrderID]*types.Order)
	}
	s.orders[order.UserID][order.ID] = order
	s.mu.Unlock()

	ob := s.getOrderBook(order.Category, order.Symbol)

	if order.TIF == constants.TIF_POST_ONLY {
		bid, _, bidOk := ob.BestBid()
		ask, _, askOk := ob.BestAsk()
		if order.Side == constants.ORDER_SIDE_BUY && askOk && order.Price >= ask {
			delete(s.orders[order.UserID], order.ID)
			return nil, nil
		}
		if order.Side == constants.ORDER_SIDE_SELL && bidOk && order.Price <= bid {
			delete(s.orders[order.UserID], order.ID)
			return nil, nil
		}
	}

	if err := s.clearing.Reserve(order.UserID, order.Symbol, order.Category, order.Side, order.Quantity, order.Price); err != nil {
		delete(s.orders[order.UserID], order.ID)
		return nil, err
	}

	var trades []*types.Trade
	var err error

	if order.TIF != constants.TIF_POST_ONLY {
		trades, err = ob.Match(order, order.Price)
		if err != nil {
			s.releaseOrder(order)
			delete(s.orders[order.UserID], order.ID)
			return nil, err
		}
	}

	for _, trade := range trades {
		s.clearing.ExecuteTrade(trade, order, nil)
		s.publishTrade(trade)
	}

	if order.Remaining() > 0 && (order.TIF == constants.TIF_GTC || order.TIF == constants.TIF_POST_ONLY) {
		ob.Add(order)
	}

	status := s.getOrderStatus(order)

	if len(trades) > 0 {
		s.publishOrderFilled(order)
	}

	if status == constants.ORDER_STATUS_FILLED || status == constants.ORDER_STATUS_CANCELED || status == constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED {
		if order.Remaining() > 0 {
			s.releaseOrder(order)
		}
		s.cancelOrder(order)
	} else {
		if order.ReduceOnly {
			s.mu.Lock()
			if s.reduceOnlyCommitment[order.UserID] == nil {
				s.reduceOnlyCommitment[order.UserID] = make(map[string]int64)
			}
			s.reduceOnlyCommitment[order.UserID][order.Symbol] += int64(order.Quantity - order.Filled)
			s.mu.Unlock()
		}
	}

	return &types.OrderResult{
		Orders:    []*types.Order{order},
		Trades:    trades,
		Filled:    order.Filled,
		Remaining: order.Remaining(),
		Status:    status,
	}, nil
}

func (s *Service) OnPriceTick(symbol string, price types.Price) {
	orderIDs := s.triggerMon.Check(price)

	for _, orderID := range orderIDs {
		s.handleTriggeredOrder(orderID, price)
	}
}

func (s *Service) handleTriggeredOrder(orderID types.OrderID, currentPrice types.Price) {
	var order *types.Order
	for _, userOrders := range s.orders {
		if o, ok := userOrders[orderID]; ok {
			order = o
			break
		}
	}

	if order == nil {
		return
	}

	s.triggerMon.Remove(orderID)

	s.cancelLinkedOCO(order)

	if order.CloseOnTrigger {
		s.handleCloseOnTrigger(order)
	} else {
		s.handleConditional(order)
	}
}

func (s *Service) cancelLinkedOCO(triggeredOrder *types.Order) {
	// OCO orders have the same OrderLinkId
	// When one triggers, cancel the other by OrderLinkId
	groupID := triggeredOrder.OrderLinkId
	if groupID <= 0 {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Find and cancel the linked OCO order
	for _, order := range s.orders[triggeredOrder.UserID] {
		if order.ID == triggeredOrder.ID {
			continue
		}
		if order.OrderLinkId == groupID && order.Status == constants.ORDER_STATUS_UNTRIGGERED {
			order.Status = constants.ORDER_STATUS_CANCELED
			order.ClosedAt = types.NowNano()
			s.triggerMon.Remove(order.ID)
			delete(s.orders[triggeredOrder.UserID], order.ID)
			break
		}
	}
}

func (s *Service) handleCloseOnTrigger(order *types.Order) {
	order.Status = constants.ORDER_STATUS_TRIGGERED
	order.UpdatedAt = types.NowNano()

	positions := s.portfolio.GetPositions(order.UserID)
	var posSize int64
	var posSide int8
	for _, p := range positions {
		if p.Symbol == order.Symbol {
			posSize = p.Size
			posSide = p.Side
			break
		}
	}
	if posSize == 0 {
		order.Status = constants.ORDER_STATUS_DEACTIVATED
		order.ClosedAt = types.NowNano()
		return
	}

	// Quantity=0 означает полное закрытие позиции
	// Quantity>0 означает частичное закрытие (ограничение сверху размером позиции)
	closeQty := order.Quantity
	if closeQty == 0 {
		closeQty = types.Quantity(posSize)
	}
	if int64(closeQty) > posSize {
		closeQty = types.Quantity(posSize)
	}

	var closeSide int8
	if posSide == constants.SIDE_LONG {
		closeSide = constants.ORDER_SIDE_SELL
	} else if posSide == constants.SIDE_SHORT {
		closeSide = constants.ORDER_SIDE_BUY
	} else {
		order.Status = constants.ORDER_STATUS_DEACTIVATED
		order.ClosedAt = types.NowNano()
		return
	}

	var orderType int8
	var tif int8
	if order.Price > 0 {
		orderType = constants.ORDER_TYPE_LIMIT
		tif = constants.TIF_GTC
	} else {
		orderType = constants.ORDER_TYPE_MARKET
		tif = constants.TIF_IOC
	}

	closeInput := &types.OrderInput{
		UserID:         order.UserID,
		Symbol:         order.Symbol,
		Category:       order.Category,
		Side:           closeSide,
		Type:           orderType,
		TIF:            tif,
		Quantity:       closeQty,
		Price:          order.Price,
		TriggerPrice:   0,
		CloseOnTrigger: false,
		ReduceOnly:     true,
	}

	s.placeNormalOrder(context.Background(), order, closeInput)
}

func (s *Service) handleConditional(order *types.Order) {
	order.Status = constants.ORDER_STATUS_TRIGGERED
	order.UpdatedAt = types.NowNano()

	positions := s.portfolio.GetPositions(order.UserID)
	var posSize int64
	for _, p := range positions {
		if p.Symbol == order.Symbol {
			posSize = p.Size
			break
		}
	}

	// Quantity=0 для TP/SL означает полное закрытие позиции на момент триггера
	// Quantity>0 означает частичное закрытие (не может превышать размер позиции)
	maxQty := order.Quantity
	if maxQty == 0 {
		maxQty = types.Quantity(posSize)
	}

	// Для reduceOnly ордеров ограничиваем размер позицией
	if order.ReduceOnly && maxQty > 0 {
		commitment := s.reduceOnlyCommitment[order.UserID][order.Symbol]
		availableForReduce := posSize - commitment
		if availableForReduce < 0 {
			availableForReduce = 0
		}
		if int64(maxQty) > availableForReduce {
			maxQty = types.Quantity(availableForReduce)
		}
	}

	twinInput := &types.OrderInput{
		UserID:         order.UserID,
		Symbol:         order.Symbol,
		Category:       order.Category,
		Side:           order.Side,
		Type:           order.Type,
		TIF:            order.TIF,
		Quantity:       maxQty,
		Price:          order.Price,
		TriggerPrice:   0,
		CloseOnTrigger: false,
		ReduceOnly:     order.ReduceOnly,
	}

	if twinInput.Quantity <= 0 {
		return
	}

	s.placeNormalOrder(context.Background(), order, twinInput)
}

func (s *Service) OnPositionUpdate(userID types.UserID, symbol string, newSize int64, newSide int8) {
	s.mu.Lock()
	defer s.mu.Unlock()

	userOrders := s.orders[userID]
	if userOrders == nil {
		return
	}

	for _, order := range userOrders {
		if order.Symbol != symbol || !order.ReduceOnly {
			continue
		}
		if order.Status != constants.ORDER_STATUS_NEW && order.Status != constants.ORDER_STATUS_PARTIALLY_FILLED {
			continue
		}
		if order.Side != newSide {
			remaining := order.Quantity - order.Filled
			if remaining > 0 {
				s.clearing.Release(order.UserID, order.Symbol, order.Category, order.Side, remaining, order.Price)
			}
			order.Status = constants.ORDER_STATUS_CANCELED
		}
	}

	s.adjustReduceOnlyOrders(userID, symbol, newSize)

	if newSize == 0 {
		s.cancelOCOGroupsForSymbol(userID, symbol)
	}
}

func (s *Service) cancelOCOGroupsForSymbol(userID types.UserID, symbol string) {
	// Cancel all OCO orders for a symbol by using OrderLinkId
	userOrders := s.orders[userID]
	if userOrders == nil {
		return
	}

	// Track which groupIDs we've already processed
	processedGroups := make(map[int64]bool)

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, order := range userOrders {
		if order.Symbol != symbol || order.OrderLinkId <= 0 {
			continue
		}
		if processedGroups[order.OrderLinkId] {
			continue
		}
		processedGroups[order.OrderLinkId] = true

		// Cancel all orders in this OCO group
		for _, linkedOrder := range userOrders {
			if linkedOrder.OrderLinkId == order.OrderLinkId && linkedOrder.Status == constants.ORDER_STATUS_UNTRIGGERED {
				linkedOrder.Status = constants.ORDER_STATUS_CANCELED
				s.triggerMon.Remove(linkedOrder.ID)
				delete(s.orders[userID], linkedOrder.ID)
			}
		}
	}
}

func (s *Service) adjustReduceOnlyOrders(userID types.UserID, symbol string, maxSize int64) {
	userOrders := s.orders[userID]
	if userOrders == nil {
		return
	}

	if maxSize <= 0 {
		for _, order := range userOrders {
			if order.Symbol != symbol || !order.ReduceOnly {
				continue
			}
			if order.Status == constants.ORDER_STATUS_NEW || order.Status == constants.ORDER_STATUS_PARTIALLY_FILLED {
				remaining := order.Quantity - order.Filled
				if remaining > 0 {
					s.clearing.Release(order.UserID, order.Symbol, order.Category, order.Side, remaining, order.Price)
				}
				order.Status = constants.ORDER_STATUS_CANCELED
			}
		}
		delete(s.reduceOnlyCommitment[userID], symbol)
		return
	}

	total := int64(0)
	for _, order := range userOrders {
		if order.Symbol != symbol || !order.ReduceOnly {
			continue
		}
		if order.Status != constants.ORDER_STATUS_NEW && order.Status != constants.ORDER_STATUS_PARTIALLY_FILLED {
			continue
		}
		total += int64(order.Quantity - order.Filled)
	}

	if total <= maxSize {
		return
	}

	remaining := maxSize
	var list []orderWithQty

	for _, order := range userOrders {
		if order.Symbol != symbol || !order.ReduceOnly {
			continue
		}
		if order.Status != constants.ORDER_STATUS_NEW && order.Status != constants.ORDER_STATUS_PARTIALLY_FILLED {
			continue
		}
		qty := int64(order.Quantity - order.Filled)
		if qty <= 0 {
			continue
		}
		list = append(list, orderWithQty{order: order, qty: qty})
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].qty > list[j].qty
	})

	for _, item := range list {
		order := item.order
		orderRemaining := item.qty
		if remaining <= 0 {
			if order.Status == constants.ORDER_STATUS_NEW {
				order.Status = constants.ORDER_STATUS_CANCELED
			}
			continue
		}
		if orderRemaining > remaining {
			order.Quantity = types.Quantity(remaining) + order.Filled
			remaining = 0
		} else {
			remaining -= orderRemaining
		}
	}

	delete(s.reduceOnlyCommitment[userID], symbol)
}

type orderWithQty struct {
	order *types.Order
	qty   int64
}

func (s *Service) getOrderStatus(order *types.Order) int8 {
	remaining := order.Remaining()
	switch order.TIF {
	case constants.TIF_GTC, constants.TIF_POST_ONLY:
		if remaining == 0 {
			return constants.ORDER_STATUS_FILLED
		}
		if order.Filled == 0 {
			return constants.ORDER_STATUS_NEW
		}
		return constants.ORDER_STATUS_PARTIALLY_FILLED
	case constants.TIF_IOC:
		if remaining == 0 {
			return constants.ORDER_STATUS_FILLED
		}
		if order.Filled == 0 {
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

func (s *Service) cancelOrder(order *types.Order) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if userOrders, ok := s.orders[order.UserID]; ok {
		delete(userOrders, order.ID)
	}
}

func (s *Service) releaseOrder(order *types.Order) {
	remaining := order.Quantity - order.Filled
	if remaining > 0 {
		s.clearing.Release(order.UserID, order.Symbol, order.Category, order.Side, remaining, order.Price)
	}
}

func (s *Service) publishPositionUpdate(trade *types.Trade) {
	if s.nats == nil {
		return
	}
	var update struct {
		UserID  types.UserID
		Symbol  string
		NewSize int64
		NewSide int8
	}
	pos := s.portfolio.GetPosition(trade.TakerID, trade.Symbol)
	update.UserID = trade.TakerID
	update.Symbol = trade.Symbol
	update.NewSize = pos.Size
	update.NewSide = pos.Side
	s.nats.PublishGob(context.Background(), messaging.PositionsEventTopic(trade.Symbol), &update)
}

func (s *Service) publishOrderEvent(order *types.Order) {
	if s.nats == nil {
		return
	}

	event := &types.OrderEvent{
		OrderID:      order.ID,
		UserID:       order.UserID,
		Symbol:       order.Symbol,
		Category:     order.Category,
		Side:         order.Side,
		Type:         order.Type,
		TIF:          order.TIF,
		Status:       order.Status,
		Price:        order.Price,
		Quantity:     order.Quantity,
		Filled:       order.Filled,
		TriggerPrice: order.TriggerPrice,
		ReduceOnly:   order.ReduceOnly,
		CreatedAt:    order.CreatedAt,
		UpdatedAt:    order.UpdatedAt,
	}
	s.nats.PublishGob(context.Background(), messaging.OrderEventTopic(order.Symbol), event)
}

func (s *Service) publishOrderFilled(order *types.Order) {
	if s.nats != nil {
		event := &types.OrderEvent{
			OrderID:   order.ID,
			UserID:    order.UserID,
			Symbol:    order.Symbol,
			Category:  order.Category,
			Side:      order.Side,
			Type:      order.Type,
			TIF:       order.TIF,
			Status:    order.Status,
			Price:     order.Price,
			Quantity:  order.Quantity,
			Filled:    order.Filled,
			CreatedAt: order.CreatedAt,
			UpdatedAt: order.UpdatedAt,
		}
		s.nats.PublishGob(context.Background(), messaging.OrderEventTopic(order.Symbol), event)
	}
}

func (s *Service) publishTrade(trade *types.Trade) {
	if s.nats != nil {
		event := &types.TradeEvent{
			TradeID:      trade.ID,
			Symbol:       trade.Symbol,
			Category:     trade.Category,
			TakerID:      trade.TakerID,
			MakerID:      trade.MakerID,
			TakerOrderID: trade.TakerOrderID,
			MakerOrderID: trade.MakerOrderID,
			Price:        trade.Price,
			Quantity:     trade.Quantity,
			ExecutedAt:   trade.ExecutedAt,
		}
		s.nats.PublishGob(context.Background(), messaging.SubjectClearingTrade, event)
	}
}

func (s *Service) getOrderBook(category int8, symbol string) *orderbook.OrderBook {
	if ob, ok := s.orderbooks[category][symbol]; ok {
		return ob
	}
	ob := orderbook.New()
	s.orderbooks[category][symbol] = ob
	return ob
}

func (s *Service) getOrderBookIfExists(category int8, symbol string) *orderbook.OrderBook {
	return s.orderbooks[category][symbol]
}

func (s *Service) WouldCross(category int8, symbol string, side int8, price types.Price) bool {
	ob := s.getOrderBookIfExists(category, symbol)
	if ob == nil {
		return false
	}
	return ob.WouldCross(side, price)
}

func (s *Service) GetOrderBook(category int8, symbol string) (bidPrice types.Price, bidQty types.Quantity, askPrice types.Price, askQty types.Quantity) {
	ob := s.getOrderBookIfExists(category, symbol)
	if ob == nil {
		return 0, 0, 0, 0
	}
	bidPrice, bidQty, _ = ob.BestBid()
	askPrice, askQty, _ = ob.BestAsk()
	return
}

func (s *Service) GetOrderBookDepth(category int8, symbol string, limit int) ([]types.Price, []types.Quantity, []types.Price, []types.Quantity) {
	ob := s.getOrderBookIfExists(category, symbol)
	if ob == nil {
		return nil, nil, nil, nil
	}
	return ob.Depth(limit)
}

func (s *Service) GetLastPrice(symbol string) types.Price {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastPrices[symbol]
}

func (s *Service) GetOrder(userID types.UserID, orderID types.OrderID) *types.Order {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if userOrders := s.orders[userID]; userOrders != nil {
		return userOrders[orderID]
	}
	return nil
}

func (s *Service) GetOrders(userID types.UserID) []*types.Order {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if userOrders := s.orders[userID]; userOrders != nil {
		orders := make([]*types.Order, 0, len(userOrders))
		for _, order := range userOrders {
			orders = append(orders, order)
		}
		return orders
	}
	return nil
}

func (s *Service) CancelOrder(ctx context.Context, userID types.UserID, orderID types.OrderID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	userOrders := s.orders[userID]
	if userOrders == nil {
		return nil
	}

	order := userOrders[orderID]
	if order == nil {
		return nil
	}

	if order.Status == constants.ORDER_STATUS_UNTRIGGERED {
		s.triggerMon.Remove(orderID)
	}

	ob := s.getOrderBookIfExists(order.Category, order.Symbol)
	if ob != nil {
		ob.Remove(order.ID)
	}
	delete(userOrders, orderID)
	if len(userOrders) == 0 {
		delete(s.orders, userID)
	}

	remaining := order.Quantity - order.Filled
	if remaining > 0 && order.Status != constants.ORDER_STATUS_UNTRIGGERED {
		s.clearing.Release(order.UserID, order.Symbol, order.Category, order.Side, remaining, order.Price)
	}

	if order.Filled > 0 {
		order.Status = constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED
	} else {
		order.Status = constants.ORDER_STATUS_CANCELED
	}
	order.UpdatedAt = types.NowNano()

	s.publishOrderCanceled(order)

	pool.PutOrder(order)

	return nil
}

func (s *Service) publishOrderCanceled(order *types.Order) {
	if s.nats == nil {
		return
	}

	event := &types.OrderEvent{
		OrderID:      order.ID,
		UserID:       order.UserID,
		Symbol:       order.Symbol,
		Category:     order.Category,
		Side:         order.Side,
		Type:         order.Type,
		TIF:          order.TIF,
		Status:       order.Status,
		Price:        order.Price,
		Quantity:     order.Quantity,
		Filled:       order.Filled,
		TriggerPrice: order.TriggerPrice,
		ReduceOnly:   order.ReduceOnly,
		UpdatedAt:    order.UpdatedAt,
	}

	s.nats.PublishGob(context.Background(), messaging.OrderEventTopic(order.Symbol), event)
}

func poolGetOrderID() uint64 {
	return uint64(types.NowNano())
}

var (
	ErrReduceOnlySpot               = errors.New("reduceOnly not allowed for SPOT")
	ErrConditionalSpot              = errors.New("conditional orders not allowed for SPOT")
	ErrCloseOnTriggerSpot           = errors.New("closeOnTrigger not allowed for SPOT")
	ErrCloseOnTriggerNoPosition     = errors.New("closeOnTrigger requires an existing position")
	ErrMarketTIF                    = errors.New("market orders must be IOC or FOK")
	ErrReduceOnlyNoPosition         = errors.New("reduceOnly not allowed without position")
	ErrReduceOnlyCommitmentExceeded = errors.New("reduceOnly commitment exceeds position")
	ErrSelfMatch                    = errors.New("self-match prevention: order would match with own order")
	ErrOCOSpot                      = errors.New("OCO orders not allowed for SPOT")
	ErrOCONoPosition                = errors.New("OCO orders require an existing position")
	ErrInvalidTriggerPrice          = errors.New("invalid trigger price: BUY trigger must be below current price, SELL trigger must be above")
	ErrInvalidQuantity              = errors.New("quantity must be greater than 0")
	ErrInvalidPrice                 = errors.New("price must be greater than or equal to 0 for LIMIT orders")
	ErrInvalidSymbol                = errors.New("invalid symbol format")
	ErrInvalidCategory              = errors.New("invalid category: must be 0 (SPOT) or 1 (LINEAR)")
	ErrInvalidSide                  = errors.New("invalid side: must be 0 (BUY) or 1 (SELL)")
	ErrInvalidOrderType             = errors.New("invalid order type: must be 0 (LIMIT) or 1 (MARKET)")
	ErrInvalidTIF                   = errors.New("invalid time in force")
	ErrInvalidStopOrderType         = errors.New("invalid stop order type")
	ErrOCOTPTriggerInvalid          = errors.New("OCO TP trigger must be > SL trigger for LONG positions")
	ErrOCOSLTriggerInvalid          = errors.New("OCO SL trigger must be < TP trigger for SHORT positions")
)
