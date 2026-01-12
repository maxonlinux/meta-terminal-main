package oms

import (
	"context"
	"sync"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/events"
	"github.com/anomalyco/meta-terminal-go/internal/messaging"
	"github.com/anomalyco/meta-terminal-go/internal/orderbook"
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
	ordersByID map[types.OrderID]*types.Order
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

	mu sync.RWMutex
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
		ordersByID:           make(map[types.OrderID]*types.Order),
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
			buf := make([]types.Match, 0, 32)
			return &buf
		}},
		triggerBufPool: sync.Pool{New: func() interface{} {
			buf := make([]*types.Order, 0, 32)
			return &buf
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
	_, _ = s.PlaceOrder(context.Background(), &input)
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
