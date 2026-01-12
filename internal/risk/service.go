package risk

import (
	"context"
	"log"
	"sync"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
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

type liquidationTask struct {
	userID     types.UserID
	symbol     string
	size       int64
	side       int8
	entryPrice int64
	leverage   int8
}

type Service struct {
	nats *messaging.NATS
	oms  OMS

	mu                sync.RWMutex
	positionsByUser   map[types.UserID]map[string]*types.Position
	positionsBySymbol map[string]map[types.UserID]*types.Position
	lastPrices        map[string]types.Price
	liquidationPool   sync.Pool
	inputPool         sync.Pool
	logLiquidations   bool
}

func New(cfg Config, portfolioService Portfolio, omsService OMS) (*Service, error) {
	n, err := messaging.New(messaging.Config{URL: cfg.NATSURL, StreamPrefix: cfg.StreamPrefix})
	if err != nil {
		return nil, err
	}

	return &Service{
		nats:              n,
		oms:               omsService,
		positionsByUser:   make(map[types.UserID]map[string]*types.Position),
		positionsBySymbol: make(map[string]map[types.UserID]*types.Position),
		lastPrices:        make(map[string]types.Price),
		liquidationPool: sync.Pool{New: func() interface{} {
			buf := make([]liquidationTask, 0, 32)
			return &buf
		}},
		inputPool: sync.Pool{New: func() interface{} { return &types.OrderInput{} }},
	}, nil
}

func (s *Service) Start(ctx context.Context) error {
	s.nats.Subscribe(ctx, messaging.SUBJECT_PRICE_TICK, "risk", s.handlePriceTick)
	s.nats.Subscribe(ctx, messaging.SUBJECT_POSITIONS_EVENT, "risk-positions", s.handlePositionUpdate)
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
	s.OnPositionUpdate(update.UserID, update.Symbol, update.NewSize, update.NewSide, update.EntryPrice, update.Leverage)
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
	s.OnPriceTick(tick.Symbol, tick.Price)
}

func (s *Service) OnPriceTick(symbol string, price types.Price) {
	s.mu.Lock()
	s.lastPrices[symbol] = price
	s.mu.Unlock()

	s.checkLiquidations(symbol, price)
}

func (s *Service) OnPositionUpdate(userID types.UserID, symbol string, newSize int64, newSide int8, entryPrice int64, leverage int8) {
	s.upsertPosition(userID, symbol, newSize, newSide, entryPrice, leverage)
}

func (s *Service) checkLiquidations(symbol string, currentPrice types.Price) {
	bufAny := s.liquidationPool.Get()
	buf, _ := bufAny.(*[]liquidationTask)
	if buf == nil {
		empty := make([]liquidationTask, 0, 32)
		buf = &empty
	}
	tasks := (*buf)[:0]

	s.mu.RLock()
	positions := s.positionsBySymbol[symbol]
	for userID, pos := range positions {
		if !s.shouldLiquidate(pos, symbol, currentPrice) {
			continue
		}
		tasks = append(tasks, liquidationTask{
			userID:     userID,
			symbol:     pos.Symbol,
			size:       pos.Size,
			side:       pos.Side,
			entryPrice: pos.EntryPrice,
			leverage:   pos.Leverage,
		})
	}
	s.mu.RUnlock()

	for i := range tasks {
		s.liquidatePosition(tasks[i])
	}
	for i := range tasks {
		tasks[i] = liquidationTask{}
	}
	*buf = tasks[:0]
	s.liquidationPool.Put(buf)
}

func (s *Service) calculateLiquidationPrice(pos *types.Position) int64 {
	if pos.Size == 0 || pos.Leverage == 0 {
		return 0
	}

	if pos.Side == constants.ORDER_SIDE_BUY {
		return pos.EntryPrice * int64(100-pos.Leverage*5) / 100
	}
	return pos.EntryPrice + pos.EntryPrice*int64(pos.Leverage*5)/100
}

func (s *Service) liquidatePosition(task liquidationTask) {
	if s.logLiquidations {
		log.Printf("risk: liquidating position user=%d symbol=%s size=%d side=%d entry=%d",
			task.userID, task.symbol, task.size, task.side, task.entryPrice)
	}

	inputAny := s.inputPool.Get()
	input, _ := inputAny.(*types.OrderInput)
	if input == nil {
		input = new(types.OrderInput)
	}
	*input = types.OrderInput{
		UserID:     task.userID,
		Symbol:     task.symbol,
		Category:   1,
		Side:       oppositeOrderSide(task.side),
		Type:       1,
		Quantity:   types.Quantity(task.size),
		Price:      0,
		ReduceOnly: true,
		TIF:        1,
	}

	result, err := s.oms.PlaceOrder(context.Background(), input)
	*input = types.OrderInput{}
	s.inputPool.Put(input)
	if err != nil {
		log.Printf("risk: liquidation order failed: %v", err)
		return
	}
	if s.logLiquidations {
		log.Printf("risk: liquidation order placed id=%d status=%d", result.Orders[0].ID, result.Orders[0].Status)
	}
}

func (s *Service) shouldLiquidate(pos *types.Position, symbol string, currentPrice types.Price) bool {
	if pos.Symbol != symbol || pos.Size == 0 {
		return false
	}
	liqPrice := s.calculateLiquidationPrice(pos)
	if liqPrice == 0 {
		return false
	}
	if pos.Side == constants.ORDER_SIDE_BUY {
		return types.Price(liqPrice) >= currentPrice
	}
	if pos.Side == constants.ORDER_SIDE_SELL {
		return types.Price(liqPrice) <= currentPrice
	}
	return false
}

func oppositeOrderSide(side int8) int8 {
	if side == constants.ORDER_SIDE_BUY {
		return constants.ORDER_SIDE_SELL
	}
	return constants.ORDER_SIDE_BUY
}

func (s *Service) UpdatePosition(userID types.UserID, pos *types.Position) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.positionsByUser[userID]; !ok {
		s.positionsByUser[userID] = make(map[string]*types.Position)
	}
	if _, ok := s.positionsBySymbol[pos.Symbol]; !ok {
		s.positionsBySymbol[pos.Symbol] = make(map[types.UserID]*types.Position)
	}
	s.positionsByUser[userID][pos.Symbol] = pos
	s.positionsBySymbol[pos.Symbol][userID] = pos
}

func (s *Service) upsertPosition(userID types.UserID, symbol string, size int64, side int8, entryPrice int64, leverage int8) {
	if size == 0 {
		s.RemovePosition(userID, symbol)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.positionsByUser[userID]; !ok {
		s.positionsByUser[userID] = make(map[string]*types.Position)
	}
	if _, ok := s.positionsBySymbol[symbol]; !ok {
		s.positionsBySymbol[symbol] = make(map[types.UserID]*types.Position)
	}
	pos := s.positionsByUser[userID][symbol]
	if pos == nil {
		pos = &types.Position{}
		s.positionsByUser[userID][symbol] = pos
		s.positionsBySymbol[symbol][userID] = pos
	} else {
		s.positionsBySymbol[symbol][userID] = pos
	}
	pos.Symbol = symbol
	pos.Size = size
	pos.Side = side
	pos.EntryPrice = entryPrice
	pos.Leverage = leverage
}

func (s *Service) RemovePosition(userID types.UserID, symbol string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if userPositions, ok := s.positionsByUser[userID]; ok {
		delete(userPositions, symbol)
	}
	if symbolPositions, ok := s.positionsBySymbol[symbol]; ok {
		delete(symbolPositions, userID)
		if len(symbolPositions) == 0 {
			delete(s.positionsBySymbol, symbol)
		}
	}
}

func (s *Service) Stop() {
	if s.nats != nil {
		s.nats.Close()
	}
}
