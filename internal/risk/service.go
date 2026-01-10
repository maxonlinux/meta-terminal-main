package risk

import (
	"context"
	"log"
	"sync"

	"github.com/anomalyco/meta-terminal-go/internal/messaging"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

type Config struct {
	NATSURL      string
	StreamPrefix string
}

type Portfolio interface {
	GetPositions(userID types.UserID) []*types.Position
	GetPosition(userID types.UserID, symbol string) *types.Position
	GetLiquidationPrice(pos *types.Position) int64
}

type OMS interface {
	PlaceOrder(ctx context.Context, input *types.OrderInput) (*types.OrderResult, error)
}

type Service struct {
	nats *messaging.NATS
	oms  OMS

	mu         sync.RWMutex
	positions  map[types.UserID]map[string]*types.Position
	lastPrices map[string]types.Price
}

func New(cfg Config, portfolioService Portfolio, omsService OMS) (*Service, error) {
	n, err := messaging.New(messaging.Config{URL: cfg.NATSURL, StreamPrefix: cfg.StreamPrefix})
	if err != nil {
		return nil, err
	}

	return &Service{
		nats:       n,
		oms:        omsService,
		positions:  make(map[types.UserID]map[string]*types.Position),
		lastPrices: make(map[string]types.Price),
	}, nil
}

func (s *Service) Start(ctx context.Context) error {
	s.nats.Subscribe(ctx, messaging.SubjectPriceTick, "risk", s.handlePriceTick)
	s.nats.Subscribe(ctx, messaging.SubjectPositionsEvent, "risk-positions", s.handlePositionUpdate)
	log.Println("risk service started")
	return nil
}

func (s *Service) handlePositionUpdate(data []byte) {
	var update struct {
		UserID     types.UserID
		Symbol     string
		NewSize    int64
		NewSide    int8
		EntryPrice int64
		Leverage   int8
	}
	if err := messaging.DecodeGob(data, &update); err != nil {
		log.Printf("risk: failed to decode position update: %v", err)
		return
	}

	pos := &types.Position{
		Symbol:     update.Symbol,
		Size:       update.NewSize,
		Side:       update.NewSide,
		EntryPrice: update.EntryPrice,
		Leverage:   update.Leverage,
	}

	if update.NewSize == 0 {
		s.RemovePosition(update.UserID, update.Symbol)
	} else {
		s.UpdatePosition(update.UserID, pos)
	}
}

func (s *Service) handlePriceTick(data []byte) {
	var tick struct {
		Symbol string
		Price  types.Price
	}
	if err := messaging.DecodeGob(data, &tick); err != nil {
		log.Printf("risk: failed to decode price tick: %v", err)
		return
	}

	s.mu.Lock()
	s.lastPrices[tick.Symbol] = tick.Price
	s.mu.Unlock()

	s.checkLiquidations(tick.Symbol, tick.Price)
}

func (s *Service) checkLiquidations(symbol string, currentPrice types.Price) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for userID, positions := range s.positions {
		for _, pos := range positions {
			if pos.Symbol != symbol {
				continue
			}
			if pos.Size == 0 {
				continue
			}

			liqPrice := s.calculateLiquidationPrice(pos)
			if liqPrice == 0 {
				continue
			}

			shouldLiquidate := false
			if pos.Side == 0 && types.Price(liqPrice) >= currentPrice {
				shouldLiquidate = true
			} else if pos.Side == 1 && types.Price(liqPrice) <= currentPrice {
				shouldLiquidate = true
			}

			if shouldLiquidate {
				s.liquidatePosition(userID, pos)
			}
		}
	}
}

func (s *Service) calculateLiquidationPrice(pos *types.Position) int64 {
	if pos.Size == 0 || pos.Leverage == 0 {
		return 0
	}

	if pos.Side == 0 {
		return pos.EntryPrice * int64(100-pos.Leverage*5) / 100
	}
	return pos.EntryPrice + pos.EntryPrice*int64(pos.Leverage*5)/100
}

func (s *Service) liquidatePosition(userID types.UserID, pos *types.Position) {
	log.Printf("risk: liquidating position user=%d symbol=%s size=%d side=%d entry=%d",
		userID, pos.Symbol, pos.Size, pos.Side, pos.EntryPrice)

	var side int8
	if pos.Side == 0 {
		side = 1
	} else {
		side = 0
	}

	input := &types.OrderInput{
		UserID:     userID,
		Symbol:     pos.Symbol,
		Category:   1,
		Side:       side,
		Type:       1,
		Quantity:   types.Quantity(pos.Size),
		Price:      0,
		ReduceOnly: true,
		TIF:        1,
	}

	result, err := s.oms.PlaceOrder(context.Background(), input)
	if err != nil {
		log.Printf("risk: liquidation order failed: %v", err)
		return
	}

	log.Printf("risk: liquidation order placed id=%d status=%d", result.Orders[0].ID, result.Orders[0].Status)
}

func (s *Service) UpdatePosition(userID types.UserID, pos *types.Position) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.positions[userID]; !ok {
		s.positions[userID] = make(map[string]*types.Position)
	}
	s.positions[userID][pos.Symbol] = pos
}

func (s *Service) RemovePosition(userID types.UserID, symbol string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.positions[userID]; ok {
		delete(s.positions[userID], symbol)
	}
}

func (s *Service) Stop() {
	if s.nats != nil {
		s.nats.Close()
	}
}
