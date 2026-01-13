package oms

import (
	"context"
	"errors"
	"sync"

	"github.com/anomalyco/meta-terminal-go/internal/actor"
	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/events"
	"github.com/anomalyco/meta-terminal-go/internal/messaging"
	"github.com/anomalyco/meta-terminal-go/internal/orderbook"
	"github.com/anomalyco/meta-terminal-go/internal/triggers"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

var (
	ErrReduceOnlySpot     = errors.New("reduceOnly not allowed for SPOT")
	ErrConditionalSpot    = errors.New("conditional orders not allowed for SPOT")
	ErrCloseOnTriggerSpot = errors.New("closeOnTrigger not allowed for SPOT")
	ErrMarketTIF          = errors.New("market orders must be IOC or FOK")
	ErrReduceOnlyNoPos    = errors.New("reduceOnly not allowed without position")
	ErrReduceOnlyCommit   = errors.New("reduceOnly commitment exceeds position")
)

type ValidationError struct {
	Code    int8
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

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

type ActorOMS struct {
	userActors     *actor.Actor
	matchingActor  *actor.Actor
	portfolio      Portfolio
	clearing       Clearing
	nats           *messaging.NATS
	triggerMon     *triggers.Monitor
	lastPrices     map[string]types.Price
	sink           events.Sink
	matchBufPool   sync.Pool
	triggerBufPool sync.Pool

	orderbooks map[int8]map[string]*orderbook.OrderBook
}

func NewActorOMS(cfg Config, portfolio Portfolio, clearing Clearing) (*ActorOMS, error) {
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

	userActors := actor.New(actor.ActorConfig{
		BufferSize:   1024,
		Workers:      1,
		StateFactory: func() any { return actor.NewMultiUserState() },
		Handler:      actor.HandleUserMessage,
	})

	matchingActor := actor.New(actor.ActorConfig{
		BufferSize:   1024,
		Workers:      1,
		StateFactory: func() any { return actor.NewMatchingActorState() },
		Handler:      actor.HandleMatchingMessage,
	})

	return &ActorOMS{
		userActors:    userActors,
		matchingActor: matchingActor,
		portfolio:     portfolio,
		clearing:      clearing,
		nats:          n,
		triggerMon:    triggers.New(),
		lastPrices:    make(map[string]types.Price),
		sink:          sink,
		orderbooks: map[int8]map[string]*orderbook.OrderBook{
			constants.CATEGORY_SPOT:   make(map[string]*orderbook.OrderBook),
			constants.CATEGORY_LINEAR: make(map[string]*orderbook.OrderBook),
		},
		matchBufPool: sync.Pool{New: func() interface{} {
			buf := make([]types.Match, 0, 32)
			return &buf
		}},
		triggerBufPool: sync.Pool{New: func() interface{} {
			buf := make([]*types.Order, 0, 32)
			return &buf
		}},
	}, nil
}

func (s *ActorOMS) Start(ctx context.Context) error {
	if s.nats == nil {
		return nil
	}
	s.nats.Subscribe(ctx, messaging.OrderPlaceTopic(""), "oms-place", s.handleOrderPlace)
	s.nats.Subscribe(ctx, messaging.OrderEventTopic(""), "oms-events", s.handleOrderEvent)
	s.nats.Subscribe(ctx, messaging.PriceTickTopic(""), "oms-price", s.handlePriceTick)
	s.nats.Subscribe(ctx, messaging.PositionsEventTopic(""), "oms-positions", s.handlePositionUpdate)
	return nil
}

func (s *ActorOMS) handleOrderEvent(data []byte) {}

func (s *ActorOMS) handleOrderPlace(data []byte) {
	var input types.OrderInput
	if err := messaging.DecodeGob(data, &input); err != nil {
		return
	}
	_, _ = s.PlaceOrder(context.Background(), &input)
}

func (s *ActorOMS) handlePriceTick(data []byte) {
	var tick struct {
		Symbol string
		Price  types.Price
	}
	if err := messaging.DecodeGob(data, &tick); err != nil {
		return
	}
	s.OnPriceTick(tick.Symbol, tick.Price)
}

func (s *ActorOMS) handlePositionUpdate(data []byte) {
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

func (s *ActorOMS) OnPositionUpdate(userID types.UserID, symbol string, newSize int64, newSide int8) {
	msg := actor.MsgPositionUpdate{
		UserID:  userID,
		Symbol:  symbol,
		NewSize: newSize,
		NewSide: newSide,
	}
	s.userActors.Send(msg)
}

func (s *ActorOMS) OnPriceTick(symbol string, price types.Price) {
	s.lastPrices[symbol] = price

	triggered := s.triggerMon.Check(price)
	for _, order := range triggered {
		order.Status = constants.ORDER_STATUS_TRIGGERED
		order.UpdatedAt = types.NowNano()

		msg := actor.MsgTriggerOrder{
			UserID: order.UserID,
			Order:  order,
		}
		s.userActors.Send(msg)
	}
}

func (s *ActorOMS) PlaceOrder(ctx context.Context, input *types.OrderInput) (*types.OrderResult, error) {
	if err := s.validateOrder(input); err != nil {
		return nil, err
	}

	resultChan := make(chan *types.OrderResult, 1)

	msg := actor.MsgPlaceOrder{
		UserID: input.UserID,
		Order:  input,
		Result: resultChan,
	}

	s.userActors.Send(msg)

	select {
	case result := <-resultChan:
		return result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *ActorOMS) validateOrder(input *types.OrderInput) error {
	if input.Category == constants.CATEGORY_SPOT {
		if input.ReduceOnly {
			return &ValidationError{Code: 6, Message: ErrReduceOnlySpot.Error()}
		}
		if input.TriggerPrice > 0 {
			return &ValidationError{Code: 6, Message: ErrConditionalSpot.Error()}
		}
		if input.CloseOnTrigger {
			return &ValidationError{Code: 6, Message: ErrCloseOnTriggerSpot.Error()}
		}
	}
	if input.Category == constants.CATEGORY_LINEAR {
		if input.Type == constants.ORDER_TYPE_MARKET && input.TIF != constants.TIF_IOC && input.TIF != constants.TIF_FOK {
			return &ValidationError{Code: 6, Message: ErrMarketTIF.Error()}
		}
	}
	return nil
}

func (s *ActorOMS) CancelOrder(userID types.UserID, orderID types.OrderID) error {
	resultChan := make(chan error, 1)

	msg := actor.MsgCancelOrder{
		UserID:  userID,
		OrderID: orderID,
		Result:  resultChan,
	}

	s.userActors.Send(msg)

	select {
	case err := <-resultChan:
		return err
	default:
		return nil
	}
}

func (s *ActorOMS) GetOrder(userID types.UserID, orderID types.OrderID) *types.Order {
	resultChan := make(chan *types.Order, 1)

	msg := actor.MsgGetOrder{
		UserID:  userID,
		OrderID: orderID,
		Result:  resultChan,
	}

	s.userActors.SendBlocking(msg)

	select {
	case order := <-resultChan:
		return order
	default:
		return nil
	}
}

func (s *ActorOMS) GetOrders(userID types.UserID) []*types.Order {
	resultChan := make(chan []*types.Order, 1)

	msg := actor.MsgGetOrders{
		UserID: userID,
		Result: resultChan,
	}

	s.userActors.SendBlocking(msg)

	select {
	case orders := <-resultChan:
		return orders
	default:
		return nil
	}
}

func (s *ActorOMS) GetPositions(userID types.UserID) []*types.Position {
	if s.portfolio != nil {
		return s.portfolio.GetPositions(userID)
	}
	return nil
}

func (s *ActorOMS) GetOrderBook(category int8, symbol string) *orderbook.OrderBook {
	return s.orderbooks[category][symbol]
}

func (s *ActorOMS) Depth(symbol string, limit int) ([]types.Price, []types.Quantity, []types.Price, []types.Quantity) {
	obSpot := s.orderbooks[constants.CATEGORY_SPOT][symbol]
	obLinear := s.orderbooks[constants.CATEGORY_LINEAR][symbol]

	if obSpot != nil {
		return obSpot.Depth(limit)
	}
	if obLinear != nil {
		return obLinear.Depth(limit)
	}
	return nil, nil, nil, nil
}

func (s *ActorOMS) MarketOrder(userID types.UserID, symbol string, side int8, category int8, quantity types.Quantity) (*types.OrderResult, error) {
	return s.PlaceOrder(context.Background(), &types.OrderInput{
		UserID:   userID,
		Symbol:   symbol,
		Category: category,
		Side:     side,
		Type:     constants.ORDER_TYPE_MARKET,
		Quantity: quantity,
	})
}

func (s *ActorOMS) LimitOrder(userID types.UserID, symbol string, side int8, category int8, quantity types.Quantity, price types.Price, tif int8, reduceOnly bool) (*types.OrderResult, error) {
	return s.PlaceOrder(context.Background(), &types.OrderInput{
		UserID:     userID,
		Symbol:     symbol,
		Category:   category,
		Side:       side,
		Type:       constants.ORDER_TYPE_LIMIT,
		Quantity:   quantity,
		Price:      price,
		TIF:        tif,
		ReduceOnly: reduceOnly,
	})
}

func (s *ActorOMS) StopOrder(userID types.UserID, symbol string, side int8, category int8, quantity types.Quantity, stopPrice types.Price, reduceOnly bool) (*types.OrderResult, error) {
	return s.PlaceOrder(context.Background(), &types.OrderInput{
		UserID:        userID,
		Symbol:        symbol,
		Category:      category,
		Side:          side,
		Type:          constants.ORDER_TYPE_LIMIT,
		Quantity:      quantity,
		TriggerPrice:  stopPrice,
		ReduceOnly:    reduceOnly,
		IsConditional: true,
	})
}

func (s *ActorOMS) TakeProfitOrder(userID types.UserID, symbol string, side int8, category int8, quantity types.Quantity, stopPrice types.Price, reduceOnly bool) (*types.OrderResult, error) {
	return s.PlaceOrder(context.Background(), &types.OrderInput{
		UserID:        userID,
		Symbol:        symbol,
		Category:      category,
		Side:          side,
		Type:          constants.ORDER_TYPE_LIMIT,
		Quantity:      quantity,
		TriggerPrice:  stopPrice,
		ReduceOnly:    reduceOnly,
		IsConditional: true,
		StopOrderType: constants.STOP_ORDER_TYPE_TAKE_PROFIT,
	})
}

func (s *ActorOMS) StopLossOrder(userID types.UserID, symbol string, side int8, category int8, quantity types.Quantity, stopPrice types.Price, reduceOnly bool) (*types.OrderResult, error) {
	return s.PlaceOrder(context.Background(), &types.OrderInput{
		UserID:        userID,
		Symbol:        symbol,
		Category:      category,
		Side:          side,
		Type:          constants.ORDER_TYPE_LIMIT,
		Quantity:      quantity,
		TriggerPrice:  stopPrice,
		ReduceOnly:    reduceOnly,
		IsConditional: true,
		StopOrderType: constants.STOP_ORDER_TYPE_STOP_LOSS,
	})
}

func (s *ActorOMS) OCOOrder(userID types.UserID, symbol string, side int8, category int8, quantity types.Quantity, takeProfitPrice types.Price, stopLossPrice types.Price) (*types.OrderResult, error) {
	return s.PlaceOrder(context.Background(), &types.OrderInput{
		UserID:   userID,
		Symbol:   symbol,
		Category: category,
		Side:     side,
		Type:     constants.ORDER_TYPE_LIMIT,
		Quantity: quantity,
		OCO: &types.OCOInput{
			Quantity: quantity,
			TakeProfit: types.OCOChildOrder{
				TriggerPrice: takeProfitPrice,
				Price:        takeProfitPrice,
				ReduceOnly:   true,
			},
			StopLoss: types.OCOChildOrder{
				TriggerPrice: stopLossPrice,
				Price:        stopLossPrice,
				ReduceOnly:   true,
			},
		},
	})
}

func (s *ActorOMS) GetLastPrice(symbol string) types.Price {
	return s.lastPrices[symbol]
}

func (s *ActorOMS) GetOrderBookDepth(symbol string, limit int) (bids []types.Price, bidQtys []types.Quantity, asks []types.Price, askQtys []types.Quantity) {
	for category := int8(0); category <= 1; category++ {
		if ob := s.orderbooks[category][symbol]; ob != nil {
			return ob.Depth(limit)
		}
	}
	return nil, nil, nil, nil
}

func (s *ActorOMS) Close() error {
	if s.nats != nil {
		s.nats.Close()
	}
	return nil
}

func (s *ActorOMS) addOrderToBook(order *types.Order) {
	if !order.IsConditional && order.Remaining() > 0 {
		ob := s.getOrCreateOrderBook(order.Category, order.Symbol)
		ob.Add(order)
	}
}

func (s *ActorOMS) getOrCreateOrderBook(category int8, symbol string) *orderbook.OrderBook {
	catMap := s.orderbooks[category]
	if catMap == nil {
		catMap = make(map[string]*orderbook.OrderBook)
		s.orderbooks[category] = catMap
	}
	ob := catMap[symbol]
	if ob == nil {
		ob = orderbook.New()
		catMap[symbol] = ob
	}
	return ob
}

func (s *ActorOMS) removeOrderFromBook(order *types.Order) bool {
	ob := s.orderbooks[order.Category][order.Symbol]
	if ob == nil {
		return false
	}
	return ob.Remove(order.ID)
}

func (s *ActorOMS) publishOrderEvent(order *types.Order) {
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

	if s.nats != nil {
		ctx := context.Background()
		_ = s.nats.PublishGob(ctx, messaging.OrderEventTopic(order.Symbol), event)
	}
}

func (s *ActorOMS) publishTrade(trade *types.Trade) {
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

	s.sink.OnTradeEvent(event)

	if s.nats != nil {
		_ = s.nats.PublishGob(context.Background(), messaging.SUBJECT_CLEARING_TRADE, trade)
	}
}
