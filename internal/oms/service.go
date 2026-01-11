package oms

import (
	"context"
	"sync"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/events"
	"github.com/anomalyco/meta-terminal-go/internal/messaging"
	"github.com/anomalyco/meta-terminal-go/internal/orderbook"
	"github.com/anomalyco/meta-terminal-go/internal/pool"
	"github.com/anomalyco/meta-terminal-go/internal/snowflake"
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
	Sink         events.Sink
}

type Service struct {
	nats       *messaging.NATS
	orderbooks map[int8]map[string]*orderbook.OrderBook
	orders     map[types.UserID]map[types.OrderID]*types.Order
	triggerMon *triggers.Monitor
	portfolio  Portfolio
	clearing   Clearing

	reduceOnlyCommitment map[types.UserID]map[string]int64
	reduceOnlyByOrder    map[types.OrderID]int64
	lastPrices           map[string]types.Price
	orderLinkIds         map[types.OrderID]int64
	linkedOrders         map[int64][]types.OrderID
	sink                 events.Sink
	matchBufPool         sync.Pool
	triggerBufPool       sync.Pool

	mu    sync.RWMutex
	posMu sync.Mutex
}

func New(cfg Config, portfolio Portfolio, clearing Clearing) (*Service, error) {
	var n *messaging.NATS
	if cfg.NATSURL != "" {
		var err error
		n, err = messaging.New(messaging.Config{URL: cfg.NATSURL, StreamPrefix: cfg.StreamPrefix})
		if err != nil {
			return nil, err
		}
	}
	sink := cfg.Sink
	if sink == nil {
		sink = events.NopSink{}
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
		reduceOnlyByOrder:    make(map[types.OrderID]int64),
		lastPrices:           make(map[string]types.Price),
		orderLinkIds:         make(map[types.OrderID]int64),
		linkedOrders:         make(map[int64][]types.OrderID),
		sink:                 sink,
		matchBufPool: sync.Pool{New: func() interface{} {
			return make([]types.Match, 0, 32)
		}},
		triggerBufPool: sync.Pool{New: func() interface{} {
			return make([]*types.Order, 0, 32)
		}},
	}, nil
}

func (s *Service) Start(ctx context.Context) error {
	if s.nats == nil {
		return nil
	}
	s.nats.Subscribe(ctx, messaging.OrderPlaceTopic(""), "oms-place", s.handleOrderPlace)
	s.nats.Subscribe(ctx, messaging.OrderEventTopic(""), "oms-events", s.handleOrderEvent)
	s.nats.Subscribe(ctx, messaging.PriceTickTopic(""), "oms-price", s.handlePriceTick)
	s.nats.Subscribe(ctx, messaging.PositionsEventTopic(""), "oms-positions", s.handlePositionUpdate)
	return nil
}

func (s *Service) handleOrderEvent(data []byte) {}

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

	if err := s.checkSelfMatch(input); err != nil {
		return nil, err
	}

	if input.TIF == constants.TIF_FOK {
		ob := s.getOrderBook(input.Category, input.Symbol)
		var limitPrice types.Price
		if input.Type == constants.ORDER_TYPE_LIMIT {
			limitPrice = input.Price
		}
		available := ob.AvailableQuantity(input.Side, limitPrice, input.Quantity)
		if available < input.Quantity {
			return nil, ErrFOKInsufficientLiquidity
		}
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
	order.StopOrderType = input.StopOrderType
	order.IsConditional = input.IsConditional
	order.OrderLinkId = 0

	if input.TriggerPrice > 0 {
		order.Status = constants.ORDER_STATUS_UNTRIGGERED
		s.triggerMon.Add(order)
		s.storeOrder(order)
		s.publishOrderEvent(order)
		result := &types.OrderResult{
			Filled:    0,
			Remaining: order.Quantity,
			Status:    order.Status,
		}
		setOrderResultOrders(result, order)
		return result, nil
	}

	return s.executeOrder(order)
}

func (s *Service) storeOrder(order *types.Order) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.orders[order.UserID]; !ok {
		s.orders[order.UserID] = make(map[types.OrderID]*types.Order)
	}
	s.orders[order.UserID][order.ID] = order
}

func (s *Service) placeOCOOrder(ctx context.Context, input *types.OrderInput) (*types.OrderResult, error) {
	if err := s.validateOCO(input); err != nil {
		return nil, err
	}

	groupID := int64(poolGetOrderID())

	tpOrder := s.createOCOChildOrder(input, groupID, constants.STOP_ORDER_TYPE_TAKE_PROFIT, input.OCO.TakeProfit)
	slOrder := s.createOCOChildOrder(input, groupID, constants.STOP_ORDER_TYPE_STOP_LOSS, input.OCO.StopLoss)

	s.storeOrder(tpOrder)
	s.storeOrder(slOrder)

	s.triggerMon.Add(tpOrder)
	s.triggerMon.Add(slOrder)

	tpResult := &types.OrderResult{
		Filled:    0,
		Remaining: types.Quantity(input.Quantity),
		Status:    tpOrder.Status,
	}
	setOrderResultOrders(tpResult, tpOrder, slOrder)

	if input.OCO.Quantity > 0 {
		tpOrder.Quantity = input.OCO.Quantity
		slOrder.Quantity = input.OCO.Quantity
	} else {
		pos := s.portfolio.GetPosition(input.UserID, input.Symbol)
		if pos != nil && pos.Size > 0 {
			tpResult.Remaining = types.Quantity(pos.Size)
		}
	}

	s.mu.Lock()
	s.orderLinkIds[tpOrder.ID] = groupID
	s.orderLinkIds[slOrder.ID] = groupID
	s.linkedOrders[groupID] = []types.OrderID{tpOrder.ID, slOrder.ID}
	s.mu.Unlock()

	s.publishOrderEvent(tpOrder)
	s.publishOrderEvent(slOrder)

	return tpResult, nil
}

func (s *Service) createOCOChildOrder(input *types.OrderInput, groupID int64, stopOrderType int8, child types.OCOChildOrder) *types.Order {
	order := pool.GetOrder()
	order.ID = types.OrderID(poolGetOrderID())
	order.UserID = input.UserID
	order.Symbol = input.Symbol
	order.Category = input.Category
	order.Side = input.Side
	order.Type = input.Type
	order.TIF = input.TIF
	order.Status = constants.ORDER_STATUS_UNTRIGGERED
	order.Price = child.Price
	order.Quantity = 0
	order.Filled = 0
	order.CreatedAt = types.NowNano()
	order.UpdatedAt = order.CreatedAt
	order.TriggerPrice = child.TriggerPrice
	order.ReduceOnly = child.ReduceOnly
	order.CloseOnTrigger = true
	order.StopOrderType = stopOrderType
	order.IsConditional = true
	order.OrderLinkId = groupID
	return order
}

func (s *Service) executeOrder(order *types.Order) (*types.OrderResult, error) {
	if order.Type == constants.ORDER_TYPE_LIMIT {
		err := s.clearing.Reserve(order.UserID, order.Symbol, order.Category, order.Side, order.Quantity, order.Price)
		if err != nil {
			s.mu.Lock()
			delete(s.orders[order.UserID], order.ID)
			s.mu.Unlock()
			pool.PutOrder(order)
			return nil, err
		}
	}

	s.storeOrder(order)

	result := &types.OrderResult{}
	setOrderResultOrders(result, order)
	s.matchOrderInto(order, result)

	order.Filled = order.Quantity - order.Remaining()
	order.UpdatedAt = types.NowNano()

	switch {
	case order.Remaining() == 0 && order.Filled > 0:
		order.Status = constants.ORDER_STATUS_FILLED
	case order.Remaining() == 0 && order.Filled == 0:
		order.Status = constants.ORDER_STATUS_CANCELED
	case order.TIF == constants.TIF_GTC || order.TIF == constants.TIF_POST_ONLY:
		ob := s.getOrderBook(order.Category, order.Symbol)
		ob.Add(order)
		order.Status = constants.ORDER_STATUS_NEW
	case order.TIF == constants.TIF_IOC || order.TIF == constants.TIF_FOK:
		order.Status = constants.ORDER_STATUS_CANCELED
		if order.Filled > 0 {
			order.Status = constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED
		}
	}
	if order.Status == constants.ORDER_STATUS_NEW {
		s.publishOrderEvent(order)
		if order.ReduceOnly && !order.IsConditional && !order.CloseOnTrigger {
			s.updateReduceOnlyCommitment(order, int64(order.Remaining()))
		}
	} else if order.Status == constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED || order.Status == constants.ORDER_STATUS_CANCELED {
		if order.ReduceOnly && !order.IsConditional && !order.CloseOnTrigger {
			s.updateReduceOnlyCommitment(order, 0)
		}
		if order.Remaining() > 0 {
			s.clearing.Release(order.UserID, order.Symbol, order.Category, order.Side, order.Remaining(), order.Price)
		}
		s.publishOrderEvent(order)
		s.mu.Lock()
		if userOrders, ok := s.orders[order.UserID]; ok {
			delete(userOrders, order.ID)
		}
		s.mu.Unlock()
	}
	result.Filled = order.Filled
	result.Remaining = order.Remaining()
	result.Status = order.Status
	return result, nil
}

func (s *Service) cancelOrder(order *types.Order) {
	if order.Status == constants.ORDER_STATUS_UNTRIGGERED {
		s.triggerMon.Remove(order.ID)
	}
	if order.ReduceOnly && !order.IsConditional && !order.CloseOnTrigger {
		s.updateReduceOnlyCommitment(order, 0)
	}

	ob := s.getOrderBookIfExists(order.Category, order.Symbol)
	if ob != nil {
		ob.Remove(order.ID)
	}

	s.mu.Lock()
	if userOrders, ok := s.orders[order.UserID]; ok {
		delete(userOrders, order.ID)
	}
	s.mu.Unlock()

	if order.Filled > 0 {
		order.Status = constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED
	} else {
		order.Status = constants.ORDER_STATUS_CANCELED
	}
	order.UpdatedAt = types.NowNano()

	s.publishOrderEvent(order)

	pool.PutOrder(order)
}

func (s *Service) OnPriceTick(symbol string, price types.Price) {
	s.mu.Lock()
	s.lastPrices[symbol] = price
	s.mu.Unlock()

	bufAny := s.triggerBufPool.Get()
	buf, _ := bufAny.([]*types.Order)
	triggered := s.triggerMon.CheckInto(price, buf)
	for _, order := range triggered {
		if order.Status == constants.ORDER_STATUS_DEACTIVATED {
			continue
		}
		if order.OrderLinkId > 0 {
			s.deactivateLinkedOrders(order)
		}

		s.handleTrigger(order)
	}
	for i := range triggered {
		triggered[i] = nil
	}
	s.triggerBufPool.Put(triggered[:0])
}

func (s *Service) handleTrigger(order *types.Order) {
	s.mu.Lock()
	delete(s.orders[order.UserID], order.ID)
	s.mu.Unlock()
	s.triggerMon.Remove(order.ID)

	childInput := s.createChildOrderInput(order)
	if childInput != nil {
		s.PlaceOrder(context.Background(), childInput)
	}
}

func (s *Service) createChildOrderInput(triggered *types.Order) *types.OrderInput {
	if triggered.CloseOnTrigger {
		pos := s.portfolio.GetPosition(triggered.UserID, triggered.Symbol)
		if pos.Size == 0 {
			return nil
		}

		qty := triggered.Quantity
		if qty == 0 {
			qty = types.Quantity(absInt64(pos.Size))
		} else if int64(qty) > absInt64(pos.Size) {
			qty = types.Quantity(absInt64(pos.Size))
		}

		var tif int8
		var price types.Price
		if triggered.Type == constants.ORDER_TYPE_MARKET {
			tif = constants.TIF_IOC
		} else {
			tif = constants.TIF_GTC
			price = triggered.Price
		}

		side := int8(constants.ORDER_SIDE_SELL)
		if pos.Side == constants.SIDE_SHORT {
			side = constants.ORDER_SIDE_BUY
		}
		return &types.OrderInput{
			UserID:     triggered.UserID,
			Symbol:     triggered.Symbol,
			Category:   triggered.Category,
			Side:       side,
			Type:       triggered.Type,
			TIF:        tif,
			Quantity:   qty,
			Price:      price,
			ReduceOnly: true,
		}
	}

	return &types.OrderInput{
		UserID:        triggered.UserID,
		Symbol:        triggered.Symbol,
		Category:      triggered.Category,
		Side:          triggered.Side,
		Type:          triggered.Type,
		TIF:           triggered.TIF,
		Quantity:      triggered.Quantity,
		Price:         triggered.Price,
		TriggerPrice:  0,
		ReduceOnly:    triggered.ReduceOnly,
		StopOrderType: triggered.StopOrderType,
	}
}

func (s *Service) deactivateLinkedOrders(triggered *types.Order) {
	s.mu.RLock()
	groupID, ok := s.orderLinkIds[triggered.ID]
	if !ok {
		s.mu.RUnlock()
		return
	}
	linkedIDs := s.linkedOrders[groupID]
	s.mu.RUnlock()

	for _, linkedID := range linkedIDs {
		if linkedID == triggered.ID {
			continue
		}

		s.mu.RLock()
		var linkedOrder *types.Order
		for _, userOrders := range s.orders {
			if order := userOrders[linkedID]; order != nil {
				linkedOrder = order
				break
			}
		}
		s.mu.RUnlock()

		if linkedOrder == nil || linkedOrder.Status != constants.ORDER_STATUS_UNTRIGGERED {
			continue
		}

		s.triggerMon.Remove(linkedOrder.ID)
		linkedOrder.Status = constants.ORDER_STATUS_DEACTIVATED
		linkedOrder.UpdatedAt = types.NowNano()
		s.publishOrderEvent(linkedOrder)
	}

	s.mu.Lock()
	delete(s.orderLinkIds, triggered.ID)
	delete(s.linkedOrders, groupID)
	s.mu.Unlock()
}

func (s *Service) publishOrderEvent(order *types.Order) {
	if s.nats == nil {
		if !s.hasSink() {
			return
		}
		s.publishOrderToSink(order)
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
		OrderLinkId:  order.OrderLinkId,
		CreatedAt:    order.CreatedAt,
		UpdatedAt:    order.UpdatedAt,
	}
	s.nats.PublishGob(context.Background(), messaging.OrderEventTopic(order.Symbol), event)
	if s.hasSink() {
		s.sink.OnOrderEvent(event)
	}
}

func (s *Service) publishOrderToSink(order *types.Order) {
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
		OrderLinkId:  order.OrderLinkId,
		CreatedAt:    order.CreatedAt,
		UpdatedAt:    order.UpdatedAt,
	}
	s.sink.OnOrderEvent(event)
}

func (s *Service) hasSink() bool {
	_, ok := s.sink.(events.NopSink)
	return !ok
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

func (s *Service) GetOrderBook(category int8, symbol string) (bidPrice types.Price, bidQty types.Quantity, askPrice types.Price, askQty types.Quantity) {
	ob := s.getOrderBookIfExists(category, symbol)
	if ob == nil {
		return 0, 0, 0, 0
	}
	bidPrice, bidQty, _ = ob.BestBid()
	askPrice, askQty, _ = ob.BestAsk()
	return
}

func (s *Service) GetLastPrice(symbol string) types.Price {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastPrices[symbol]
}

func (s *Service) GetOrderBookDepth(category int8, symbol string, limit int) ([]types.Price, []types.Quantity, []types.Price, []types.Quantity) {
	ob := s.getOrderBookIfExists(category, symbol)
	if ob == nil {
		return nil, nil, nil, nil
	}
	return ob.Depth(limit)
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

	s.publishOrderEvent(order)

	pool.PutOrder(order)

	return nil
}

func (s *Service) OnPositionUpdate(userID types.UserID, symbol string, newSize int64, newSide int8) {
	s.posMu.Lock()
	defer s.posMu.Unlock()
	var toDeactivate []*types.Order
	var groups []int64
	var reduceOnlyOrders []*types.Order
	var closeOnTriggerOrders []*types.Order
	allowed := absInt64(newSize)

	s.mu.Lock()
	if userOrders, ok := s.orders[userID]; ok {
		for _, order := range userOrders {
			if order.Symbol != symbol {
				continue
			}
			if order.Status != constants.ORDER_STATUS_UNTRIGGERED {
				if order.ReduceOnly && order.Status == constants.ORDER_STATUS_NEW {
					reduceOnlyOrders = append(reduceOnlyOrders, order)
				}
				continue
			}
			if order.OrderLinkId <= 0 && !order.CloseOnTrigger {
				continue
			}
			if order.CloseOnTrigger && order.Quantity > 0 {
				closeOnTriggerOrders = append(closeOnTriggerOrders, order)
			}
			toDeactivate = append(toDeactivate, order)
			if order.OrderLinkId > 0 {
				groups = append(groups, order.OrderLinkId)
			}
		}
	}
	s.mu.Unlock()

	if len(reduceOnlyOrders) > 0 {
		s.adjustReduceOnlyOrders(userID, symbol, reduceOnlyOrders, allowed)
	}
	if len(closeOnTriggerOrders) > 0 {
		s.adjustCloseOnTriggerOrders(closeOnTriggerOrders, allowed)
	}

	if newSize != 0 {
		return
	}

	for _, order := range toDeactivate {
		s.triggerMon.Remove(order.ID)
		order.Status = constants.ORDER_STATUS_DEACTIVATED
		order.UpdatedAt = types.NowNano()
		s.publishOrderEvent(order)
	}

	if len(groups) == 0 {
		return
	}

	s.mu.Lock()
	for _, groupID := range groups {
		if ids := s.linkedOrders[groupID]; len(ids) > 0 {
			for _, id := range ids {
				delete(s.orderLinkIds, id)
			}
		}
		delete(s.linkedOrders, groupID)
	}
	s.mu.Unlock()
}

func poolGetOrderID() uint64 {
	return uint64(snowflake.Next())
}

func setOrderResultOrders(result *types.OrderResult, orders ...*types.Order) {
	if result == nil {
		return
	}
	if len(orders) <= len(result.OrdersBuf) {
		result.Orders = result.OrdersBuf[:len(orders)]
		copy(result.Orders, orders)
		return
	}
	result.Orders = make([]*types.Order, len(orders))
	copy(result.Orders, orders)
}

func (s *Service) updateReduceOnlyCommitment(order *types.Order, newValue int64) {
	if order == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	old := s.reduceOnlyByOrder[order.ID]
	if newValue == old {
		return
	}
	if newValue == 0 {
		delete(s.reduceOnlyByOrder, order.ID)
	} else {
		s.reduceOnlyByOrder[order.ID] = newValue
	}
	if _, ok := s.reduceOnlyCommitment[order.UserID]; !ok {
		s.reduceOnlyCommitment[order.UserID] = make(map[string]int64)
	}
	s.reduceOnlyCommitment[order.UserID][order.Symbol] += newValue - old
	if s.reduceOnlyCommitment[order.UserID][order.Symbol] < 0 {
		s.reduceOnlyCommitment[order.UserID][order.Symbol] = 0
	}
}

func (s *Service) adjustReduceOnlyOrders(userID types.UserID, symbol string, orders []*types.Order, allowed int64) {
	if allowed < 0 {
		allowed = 0
	}
	for _, order := range orders {
		if allowed == 0 {
			allowed = 0
		}
		remaining := int64(order.Remaining())
		newRemaining := remaining
		if remaining > allowed {
			newRemaining = allowed
		}
		allowed -= newRemaining
		if newRemaining != remaining {
			ob := s.getOrderBook(order.Category, order.Symbol)
			_ = ob.Adjust(order.ID, types.Quantity(newRemaining))
			order.UpdatedAt = types.NowNano()
		}
		s.updateReduceOnlyCommitment(order, newRemaining)
	}
}

func (s *Service) adjustCloseOnTriggerOrders(orders []*types.Order, allowed int64) {
	if allowed < 0 {
		allowed = 0
	}
	for _, order := range orders {
		if order.Quantity == 0 {
			continue
		}
		remaining := int64(order.Quantity)
		newRemaining := remaining
		if remaining > allowed {
			newRemaining = allowed
		}
		allowed -= newRemaining
		if newRemaining != remaining {
			order.Quantity = types.Quantity(newRemaining)
			order.UpdatedAt = types.NowNano()
		}
	}
}
