package oms

import (
	"context"
	"encoding/binary"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/id"
	"github.com/anomalyco/meta-terminal-go/internal/messaging"
	"github.com/anomalyco/meta-terminal-go/internal/orderbook"
	"github.com/anomalyco/meta-terminal-go/internal/persistence"
	"github.com/anomalyco/meta-terminal-go/internal/pool"
	"github.com/anomalyco/meta-terminal-go/internal/registry"
	"github.com/anomalyco/meta-terminal-go/internal/triggers"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

type Config struct {
	NATSURL      string
	StreamPrefix string
	Shard        string
	Snapshots    *persistence.SnapshotStore
}

type OrderInput struct {
	UserID         types.UserID
	Symbol         string
	Category       int8
	Side           int8
	Type           int8
	TIF            int8
	Qty            int64
	Price          int64
	TriggerPrice   int64
	CloseOnTrigger bool
	ReduceOnly     bool
	Leverage       int8
}

type OrderResult struct {
	OrderID types.OrderID
	Status  int8
	Filled  int64
	Trades  []TradeResult
}

type TradeResult struct {
	TradeID id.TradeID
	Price   int64
	Qty     int64
	MakerID types.UserID
}

type Service struct {
	nats                 *messaging.NATS
	snapshots            *persistence.SnapshotStore
	shard                string
	orderbooks           map[int8]*orderbook.OrderBook
	orders               map[types.UserID]map[types.OrderID]*types.Order
	triggerMonitor       *triggers.Monitor
	registry             *registry.Registry
	reduceOnlyCommitment map[types.UserID]map[string]int64
	mu                   sync.RWMutex
}

func New(cfg Config, reg *registry.Registry) (*Service, error) {
	n, err := messaging.New(messaging.Config{URL: cfg.NATSURL, StreamPrefix: cfg.StreamPrefix})
	if err != nil {
		return nil, err
	}

	return &Service{
		nats:      n,
		snapshots: cfg.Snapshots,
		shard:     cfg.Shard,
		orderbooks: map[int8]*orderbook.OrderBook{
			constants.CATEGORY_SPOT:   orderbook.New(),
			constants.CATEGORY_LINEAR: orderbook.New(),
		},
		orders:               make(map[types.UserID]map[types.OrderID]*types.Order),
		triggerMonitor:       triggers.New(),
		registry:             reg,
		reduceOnlyCommitment: make(map[types.UserID]map[string]int64),
	}, nil
}

func (s *Service) Start(ctx context.Context) error {
	// Subscribe to order placement (new orders from Gateway)
	s.nats.Subscribe(ctx, messaging.OrderPlaceTopic(s.shard), "oms-"+s.shard+"-place", s.handleOrderPlace)
	// Subscribe to own events (for status updates, logs)
	s.nats.Subscribe(ctx, messaging.OrderEventTopic(s.shard), "oms-"+s.shard, s.handleOrderEvent)
	// Price ticks for triggers
	s.nats.Subscribe(ctx, messaging.PriceTickTopic(s.shard), "oms-price-"+s.shard, s.handlePriceTick)
	// Position updates for reduce-only
	s.nats.Subscribe(ctx, messaging.PositionsEventTopic(s.shard), "oms-"+s.shard+"-positions", s.handlePositionUpdate)
	log.Println("oms started for shard:", s.shard)
	return nil
}

func (s *Service) handleOrderEvent(data []byte) {
	log.Printf("oms received: %s", string(data))
}

func (s *Service) handleOrderPlace(data []byte) {
	// Decode Gob-encoded OrderInput
	var input types.OrderInput
	if err := messaging.DecodeGob(data, &input); err != nil {
		log.Printf("oms: gob decode error: %v", err)
		return
	}
	// Validate symbol matches shard
	if input.Symbol != s.shard {
		log.Printf("oms: symbol mismatch: %s != %s", input.Symbol, s.shard)
		return
	}
	// Convert to local OrderInput (for compatibility with existing PlaceOrder)
	localInput := &OrderInput{
		UserID:         input.UserID,
		Symbol:         input.Symbol,
		Category:       input.Category,
		Side:           input.Side,
		Type:           input.Type,
		TIF:            input.TIF,
		Qty:            int64(input.Quantity),
		Price:          int64(input.Price),
		TriggerPrice:   int64(input.TriggerPrice),
		CloseOnTrigger: input.CloseOnTrigger,
		ReduceOnly:     input.ReduceOnly,
		Leverage:       input.Leverage,
	}
	_, err := s.PlaceOrder(context.Background(), localInput)
	if err != nil {
		log.Printf("oms: place order error: %v", err)
	}
}

func (s *Service) handlePriceTick(data []byte) {
	if len(data) < 9 {
		return
	}
	price := types.Price(int64(binary.LittleEndian.Uint64(data[1:9])))
	symbolLen := int(data[9])
	if len(data) < 10+symbolLen {
		return
	}
	symbol := string(data[10 : 10+symbolLen])

	s.registry.SetPrice(symbol, registry.PriceTick{
		Price:     int64(price),
		Timestamp: int64(types.NowNano()),
	})

	log.Printf("oms price tick: %s @ %d", symbol, price)
}

func (s *Service) handlePositionUpdate(data []byte) {
	if len(data) < 10 {
		return
	}
	msgType := data[0]
	if msgType != 0x02 {
		return
	}

	offset := 1
	userID := types.UserID(binary.LittleEndian.Uint64(data[offset:]))
	offset += 8
	symbolLen := int(data[offset])
	offset++
	if len(data) < offset+symbolLen+9 {
		return
	}
	symbol := string(data[offset : offset+symbolLen])
	offset += symbolLen
	newSize := int64(binary.LittleEndian.Uint64(data[offset:]))
	offset += 8
	newSide := int8(data[offset])

	s.mu.Lock()
	defer s.mu.Unlock()

	userOrders := s.orders[userID]
	if userOrders == nil {
		return
	}

	cancelledOrderIDs := make([]types.OrderID, 0)

	for _, order := range userOrders {
		if order.Symbol != symbol || !order.ReduceOnly {
			continue
		}
		if order.Status != constants.ORDER_STATUS_NEW && order.Status != constants.ORDER_STATUS_PARTIALLY_FILLED {
			continue
		}
		if order.Side != newSide {
			order.Status = constants.ORDER_STATUS_CANCELED
			cancelledOrderIDs = append(cancelledOrderIDs, order.ID)
		}
	}

	for _, orderID := range cancelledOrderIDs {
		s.publishOrderCancel(userID, symbol, orderID)
	}

	s.adjustReduceOnlyOrders(userID, symbol, newSize)
}

func (s *Service) publishOrderCancel(userID types.UserID, symbol string, orderID types.OrderID) {
	if s.nats == nil {
		return
	}
	buf := make([]byte, 0, 8+len(symbol)+8)
	buf = binary.LittleEndian.AppendUint64(buf, uint64(userID))
	buf = append(buf, byte(len(symbol)))
	buf = append(buf, symbol...)
	buf = binary.LittleEndian.AppendUint64(buf, uint64(orderID))

	s.nats.PublishBytes(context.Background(), messaging.OrderCancelTopic(symbol), buf)
}

func (s *Service) publishOrderTrim(userID types.UserID, symbol string, orderID types.OrderID, newQty int64) {
	if s.nats == nil {
		return
	}
	buf := make([]byte, 0, 8+len(symbol)+8+8)
	buf = binary.LittleEndian.AppendUint64(buf, uint64(userID))
	buf = append(buf, byte(len(symbol)))
	buf = append(buf, symbol...)
	buf = binary.LittleEndian.AppendUint64(buf, uint64(orderID))
	buf = binary.LittleEndian.AppendUint64(buf, uint64(newQty))

	s.nats.PublishBytes(context.Background(), messaging.OrderTrimTopic(symbol), buf)
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
				order.Status = constants.ORDER_STATUS_CANCELED
				s.publishOrderCancel(userID, symbol, order.ID)
			}
		}
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
	cancelledOrderIDs := make([]types.OrderID, 0)
	trimmedOrders := make(map[types.OrderID]int64)

	// Collect reduce-only orders into a slice and sort by quantity descending (largest first)
	// to ensure deterministic trimming regardless of map iteration order.
	type orderWithQty struct {
		order *types.Order
		qty   int64
	}
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
	// Sort by qty descending (largest first)
	sort.Slice(list, func(i, j int) bool {
		return list[i].qty > list[j].qty
	})

	for _, item := range list {
		order := item.order
		orderRemaining := item.qty
		if remaining <= 0 {
			if order.Status == constants.ORDER_STATUS_NEW {
				order.Status = constants.ORDER_STATUS_CANCELED
				cancelledOrderIDs = append(cancelledOrderIDs, order.ID)
			}
			continue
		}
		if orderRemaining > remaining {
			order.Quantity = types.Quantity(remaining) + order.Filled
			trimmedOrders[order.ID] = remaining + int64(order.Filled)
			remaining = 0
		} else {
			remaining -= orderRemaining
		}
	}

	for _, orderID := range cancelledOrderIDs {
		s.publishOrderCancel(userID, symbol, orderID)
	}

	for orderID, newQty := range trimmedOrders {
		s.publishOrderTrim(userID, symbol, orderID, newQty)
	}
}

func (s *Service) getPositionSize(userID types.UserID, symbol string) int64 {
	if s.nats == nil {
		return 1000
	}
	buf := make([]byte, 0, 10+len(symbol))
	buf = append(buf, 0x03)
	buf = binary.LittleEndian.AppendUint64(buf, uint64(userID))
	buf = append(buf, byte(len(symbol)))
	buf = append(buf, symbol...)

	reply, err := s.nats.RequestReply(context.Background(), messaging.SubjectPortfolioGetPos, buf, 5*time.Second)
	if err != nil {
		return 0
	}
	if len(reply) < 9 {
		return 0
	}
	return int64(binary.LittleEndian.Uint64(reply))
}

func (s *Service) validateOrder(input *OrderInput) error {
	if input.Category == constants.CATEGORY_SPOT {
		if input.ReduceOnly {
			return &ValidationError{Code: 6, Message: "reduceOnly not allowed for SPOT"}
		}
		if input.TriggerPrice > 0 {
			return &ValidationError{Code: 6, Message: "conditional orders not allowed for SPOT"}
		}
		if input.CloseOnTrigger {
			return &ValidationError{Code: 6, Message: "closeOnTrigger not allowed for SPOT"}
		}
	}
	if input.Category == constants.CATEGORY_LINEAR {
		if input.Type == constants.ORDER_TYPE_MARKET && input.TIF != constants.TIF_IOC && input.TIF != constants.TIF_FOK {
			return &ValidationError{Code: 6, Message: "market orders must be IOC or FOK"}
		}
		if input.ReduceOnly {
			posSize := s.getPositionSize(input.UserID, input.Symbol)
			if posSize <= 0 {
				return &ValidationError{Code: 6, Message: "reduceOnly not allowed without position"}
			}
			commitment := s.reduceOnlyCommitment[input.UserID][input.Symbol]
			maxAllowed := posSize - commitment
			if maxAllowed <= 0 {
				return &ValidationError{Code: 6, Message: "reduceOnly commitment exceeds position"}
			}
			if input.Qty > maxAllowed {
				input.Qty = maxAllowed
			}
		}
	}
	return nil
}

func (s *Service) PlaceOrder(ctx context.Context, input *OrderInput) (*OrderResult, error) {
	if err := s.validateOrder(input); err != nil {
		return nil, err
	}

	orderId := id.NewOrderID()

	order := pool.GetOrder()
	order.ID = types.OrderID(orderId)
	order.UserID = input.UserID
	order.Symbol = input.Symbol
	order.Category = input.Category
	order.Side = input.Side
	order.Type = input.Type
	order.TIF = input.TIF
	order.Status = constants.ORDER_STATUS_NEW
	order.Price = types.Price(input.Price)
	order.Quantity = types.Quantity(input.Qty)
	order.Filled = 0
	order.CreatedAt = types.NowNano()
	order.UpdatedAt = order.CreatedAt
	order.TriggerPrice = types.Price(input.TriggerPrice)
	order.CloseOnTrigger = input.CloseOnTrigger
	order.ReduceOnly = input.ReduceOnly
	order.Leverage = input.Leverage

	if input.TriggerPrice > 0 {
		return s.placeConditionalOrder(ctx, order, input)
	}

	return s.placeNormalOrder(ctx, order, input)
}

func (s *Service) placeConditionalOrder(ctx context.Context, order *types.Order, input *OrderInput) (*OrderResult, error) {
	order.Status = constants.ORDER_STATUS_UNTRIGGERED

	s.mu.Lock()
	if s.orders[order.UserID] == nil {
		s.orders[order.UserID] = make(map[types.OrderID]*types.Order)
	}
	s.orders[order.UserID][order.ID] = order
	s.mu.Unlock()

	s.triggerMonitor.Add(order)

	log.Printf("conditional order placed: %d symbol=%s trigger=%d",
		order.ID, order.Symbol, order.TriggerPrice)

	return &OrderResult{
		OrderID: types.OrderID(order.ID),
		Status:  constants.ORDER_STATUS_UNTRIGGERED,
		Filled:  0,
		Trades:  nil,
	}, nil
}

func (s *Service) placeNormalOrder(ctx context.Context, order *types.Order, input *OrderInput) (*OrderResult, error) {
	s.mu.Lock()
	if s.orders[order.UserID] == nil {
		s.orders[order.UserID] = make(map[types.OrderID]*types.Order)
	}
	s.orders[order.UserID][order.ID] = order
	s.mu.Unlock()

	ob := s.getOrderBook(input.Category)

	var limitPrice types.Price
	if input.Type == constants.ORDER_TYPE_LIMIT {
		limitPrice = types.Price(input.Price)
	}

	if input.TIF == constants.TIF_POST_ONLY {
		if ob.WouldCross(input.Side, limitPrice) {
			pool.PutOrder(order)
			return nil, nil
		}
	}

	s.publishReserveRequest(ctx, order.UserID, order.Symbol, order.Category, order.Side, order.Quantity, order.Price, order.Leverage)

	var matches []types.Match
	var err error

	if input.TIF != constants.TIF_POST_ONLY {
		matches, err = ob.Match(order, limitPrice)
		if err != nil {
			pool.PutOrder(order)
			return nil, err
		}
	}

	for _, m := range matches {
		s.publishTrade(m.Trade)
	}

	if order.Remaining() > 0 && (input.TIF == constants.TIF_GTC || input.TIF == constants.TIF_POST_ONLY) {
		ob.AddResting(order)
	}

	status := s.getOrderStatus(order, matches)

	if len(matches) > 0 {
		s.publishOrderFilled(ctx, order, matches)
	}

	if status == constants.ORDER_STATUS_FILLED || status == constants.ORDER_STATUS_CANCELED || status == constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED {
		s.CancelOrder(ctx, order.UserID, types.OrderID(order.ID))
	}

	trades := make([]TradeResult, len(matches))
	for i, m := range matches {
		trades[i] = TradeResult{
			TradeID: id.TradeID(m.Trade.ID),
			Price:   int64(m.Trade.Price),
			Qty:     int64(m.Trade.Quantity),
			MakerID: m.Trade.MakerID,
		}
	}

	log.Printf("order placed: %d filled=%d trades=%d", order.ID, order.Filled, len(matches))
	return &OrderResult{
		OrderID: types.OrderID(order.ID),
		Status:  status,
		Filled:  int64(order.Filled),
		Trades:  trades,
	}, nil
}

func (s *Service) OnPriceTick(price types.Price) {
	s.registry.SetPrice(s.shard, registry.PriceTick{
		Price:     int64(price),
		Timestamp: int64(types.NowNano()),
	})

	orderIDs := s.triggerMonitor.Check(price)

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

	s.triggerMonitor.Remove(orderID)

	if order.CloseOnTrigger {
		s.handleCloseOnTrigger(order)
	} else {
		s.handleConditional(order)
	}
}

func (s *Service) handleCloseOnTrigger(order *types.Order) {
	order.Status = constants.ORDER_STATUS_TRIGGERED
	order.UpdatedAt = types.NowNano()

	posSize := s.getPositionSize(order.UserID, order.Symbol)
	if posSize == 0 {
		log.Printf("oms: closeOnTrigger but no position for user=%d symbol=%s", order.UserID, order.Symbol)
		return
	}

	side := order.Side
	if order.Side == constants.ORDER_SIDE_BUY {
		side = constants.ORDER_SIDE_SELL
	} else {
		side = constants.ORDER_SIDE_BUY
	}

	closeInput := &OrderInput{
		UserID:         order.UserID,
		Symbol:         order.Symbol,
		Category:       order.Category,
		Side:           side,
		Type:           constants.ORDER_TYPE_MARKET,
		TIF:            constants.TIF_IOC,
		Qty:            posSize,
		Price:          0,
		TriggerPrice:   0,
		CloseOnTrigger: false,
		ReduceOnly:     true,
		Leverage:       order.Leverage,
	}

	s.placeNormalOrder(context.Background(), order, closeInput)
}

func (s *Service) handleConditional(order *types.Order) {
	order.Status = constants.ORDER_STATUS_TRIGGERED
	order.UpdatedAt = types.NowNano()

	posSize := s.getPositionSize(order.UserID, order.Symbol)
	maxQty := posSize
	if order.ReduceOnly && maxQty > 0 {
		commitment := s.reduceOnlyCommitment[order.UserID][order.Symbol]
		maxQty = posSize - commitment
		if maxQty < 0 {
			maxQty = 0
		}
	}

	twinInput := &OrderInput{
		UserID:         order.UserID,
		Symbol:         order.Symbol,
		Category:       order.Category,
		Side:           order.Side,
		Type:           order.Type,
		TIF:            order.TIF,
		Qty:            int64(order.Quantity),
		Price:          int64(order.Price),
		TriggerPrice:   0,
		CloseOnTrigger: false,
		ReduceOnly:     order.ReduceOnly,
		Leverage:       order.Leverage,
	}

	if order.ReduceOnly && maxQty > 0 && twinInput.Qty > maxQty {
		twinInput.Qty = maxQty
	}

	if twinInput.Qty <= 0 {
		log.Printf("oms: conditional order %d skipped - reduceOnly qty=0", order.ID)
		return
	}

	s.placeNormalOrder(context.Background(), order, twinInput)
}

func (s *Service) getOrderStatus(order *types.Order, matches []types.Match) int8 {
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

func (s *Service) publishOrderFilled(ctx context.Context, order *types.Order, matches []types.Match) {
	if s.nats == nil {
		return
	}
	subject := messaging.OrderEventTopic(order.Symbol)
	event := types.OrderEvent{
		OrderID: order.ID,
		UserID:  order.UserID,
		Symbol:  order.Symbol,
		Status:  order.Status,
		Filled:  int64(order.Filled),
		Trades:  matches,
	}
	_ = s.nats.PublishGob(ctx, subject, &event)
}

func (s *Service) publishTrade(trade *types.Trade) {
	if s.nats == nil {
		return
	}
	_ = s.nats.PublishGob(context.Background(), messaging.SubjectClearingTrade, trade)
}

func (s *Service) publishReserveRequest(ctx context.Context, userID types.UserID, symbol string, category, side int8, qty types.Quantity, price types.Price, leverage int8) error {
	if s.nats == nil {
		return nil
	}
	buf := make([]byte, 0, 10+len(symbol)+8+1+1+8+8+1)
	buf = append(buf, 0x01)
	buf = binary.LittleEndian.AppendUint64(buf, uint64(userID))
	buf = append(buf, byte(len(symbol)))
	buf = append(buf, symbol...)
	buf = append(buf, byte(category))
	buf = append(buf, byte(side))
	buf = binary.LittleEndian.AppendUint64(buf, uint64(qty))
	buf = binary.LittleEndian.AppendUint64(buf, uint64(price))
	buf = append(buf, byte(leverage))

	reply, err := s.nats.RequestReply(ctx, messaging.SubjectClearingReserve, buf, 5*time.Second)
	if err != nil {
		return err
	}
	if len(reply) == 0 || reply[0] != 0x01 {
		return &ValidationError{Code: 1, Message: "insufficient balance"}
	}
	return nil
}

func (s *Service) publishReleaseRequest(ctx context.Context, userID types.UserID, symbol string, category, side int8, qty types.Quantity, price types.Price, leverage int8) {
	if s.nats == nil {
		return
	}
	buf := make([]byte, 0, 10+len(symbol)+8+1+1+8+8+1)
	buf = append(buf, 0x02)
	buf = binary.LittleEndian.AppendUint64(buf, uint64(userID))
	buf = append(buf, byte(len(symbol)))
	buf = append(buf, symbol...)
	buf = append(buf, byte(category))
	buf = append(buf, byte(side))
	buf = binary.LittleEndian.AppendUint64(buf, uint64(qty))
	buf = binary.LittleEndian.AppendUint64(buf, uint64(price))
	buf = append(buf, byte(leverage))

	s.nats.PublishBytes(ctx, messaging.SubjectClearingRelease, buf)
}

func (s *Service) CancelOrder(ctx context.Context, userID types.UserID, orderID types.OrderID) error {
	userOrders := s.orders[userID]
	if userOrders == nil {
		return nil
	}

	order := userOrders[orderID]
	if order == nil {
		return nil
	}

	if order.Status == constants.ORDER_STATUS_UNTRIGGERED {
		s.triggerMonitor.Remove(orderID)
	}

	ob := s.getOrderBook(order.Category)

	s.mu.Lock()
	ob.RemoveResting(order.ID)
	delete(userOrders, orderID)
	if len(userOrders) == 0 {
		delete(s.orders, userID)
	}
	s.mu.Unlock()

	remaining := int64(order.Quantity - order.Filled)
	if remaining > 0 {
		s.publishReleaseRequest(ctx, userID, order.Symbol, order.Category, order.Side, types.Quantity(remaining), order.Price, order.Leverage)
	}

	if order.Filled > 0 {
		order.Status = constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED
	} else {
		order.Status = constants.ORDER_STATUS_CANCELED
	}
	order.UpdatedAt = types.NowNano()
	pool.PutOrder(order)

	subject := messaging.OrderEventTopic(order.Symbol)
	if s.nats != nil {
		_ = s.nats.PublishGob(ctx, subject, &types.OrderCancelled{
			OrderID: orderID,
			Reason:  "user",
		})
	}

	return nil
}

func (s *Service) getOrderBook(category int8) *orderbook.OrderBook {
	if ob, ok := s.orderbooks[category]; ok {
		return ob
	}
	return nil
}

func (s *Service) GetOrderBook(category int8) (bid, ask []int64) {
	ob := s.getOrderBook(category)
	if ob == nil {
		return nil, nil
	}
	bidPrice, bidQty, ok := ob.BestBid()
	if ok {
		bid = []int64{int64(bidPrice), int64(bidQty)}
	}
	askPrice, askQty, ok := ob.BestAsk()
	if ok {
		ask = []int64{int64(askPrice), int64(askQty)}
	}
	return
}

func (s *Service) GetOrder(userID types.UserID, orderID types.OrderID) *types.Order {
	if userOrders := s.orders[userID]; userOrders != nil {
		return userOrders[orderID]
	}
	return nil
}

func (s *Service) Close() {
	s.nats.Close()
}

type ValidationError struct {
	Code    int8
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}
