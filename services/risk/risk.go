package risk

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/anomalyco/meta-terminal-go/internal/messaging"
)

type Config struct {
	NATSURL         string `env:"NATS_URL" default:"nats://localhost:4222"`
	StreamPrefix    string `env:"STREAM_PREFIX" default:"risk"`
	MaxPositionSize int64  `env:"MAX_POSITION_SIZE" default:"1000000"`
	MaxOrderQty     int64  `env:"MAX_ORDER_QTY" default:"10000"`
	MaxDailyLoss    int64  `env:"MAX_DAILY_LOSS" default:"1000000"`
	OrdersPerMinute int    `env:"ORDERS_PER_MINUTE" default:"100"`
	MarginBufferPct int64  `env:"MARGIN_BUFFER_PCT" default:"10"`
}

type ValidationResult struct {
	Valid   bool   `json:"valid"`
	Code    int8   `json:"code"`
	Message string `json:"message"`
}

const (
	VALIDATION_OK                  = 0
	VALIDATION_INSUFFICIENT_MARGIN = 1
	VALIDATION_POSITION_LIMIT      = 2
	VALIDATION_ORDER_LIMIT         = 3
	VALIDATION_RATE_LIMIT          = 4
	VALIDATION_DAILY_LOSS          = 5
	VALIDATION_REJECTED            = 6
)

type Service struct {
	cfg        Config
	nats       *messaging.NATS
	userLimits map[uint64]*UserLimits
	mu         sync.RWMutex

	orderCounts map[uint64][]int64
}

type UserLimits struct {
	UserID          uint64
	Positions       map[string]int64
	DailyPnL        int64
	OrderTimestamps []int64
}

func New(cfg Config) (*Service, error) {
	n, err := messaging.New(messaging.Config{
		URL:          cfg.NATSURL,
		StreamPrefix: cfg.StreamPrefix,
	})
	if err != nil {
		return nil, err
	}

	return &Service{
		cfg:         cfg,
		nats:        n,
		userLimits:  make(map[uint64]*UserLimits),
		orderCounts: make(map[uint64][]int64),
	}, nil
}

func (s *Service) Start(ctx context.Context) error {
	s.nats.Subscribe(ctx, "orders.>", "risk-orders", s.handleOrderEvent)
	s.nats.Subscribe(ctx, "portfolio.>", "risk-portfolio", s.handlePortfolioEvent)
	log.Println("risk service started")
	return nil
}

func (s *Service) handleOrderEvent(data []byte) {
	log.Printf("risk received order event: %s", string(data))
}

func (s *Service) handlePortfolioEvent(data []byte) {
	log.Printf("risk received portfolio event: %s", string(data))
}

func (s *Service) ValidateOrder(ctx context.Context, userID uint64, symbol string, side int8, qty int64, price int64, leverage int8) *ValidationResult {
	if qty > s.cfg.MaxOrderQty {
		return &ValidationResult{
			Valid:   false,
			Code:    VALIDATION_ORDER_LIMIT,
			Message: "order quantity exceeds limit",
		}
	}

	if err := s.checkRateLimit(userID); err != nil {
		return err
	}

	if err := s.checkPositionLimit(userID, symbol, side, qty); err != nil {
		return err
	}

	return &ValidationResult{
		Valid:   true,
		Code:    VALIDATION_OK,
		Message: "ok",
	}
}

func (s *Service) checkRateLimit(userID uint64) *ValidationResult {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UnixNano()
	oneMinuteAgo := now - 60*1000000000

	timestamps := s.orderCounts[userID]
	var validTimestamps []int64

	for _, ts := range timestamps {
		if ts > oneMinuteAgo {
			validTimestamps = append(validTimestamps, ts)
		}
	}

	if len(validTimestamps) >= s.cfg.OrdersPerMinute {
		return &ValidationResult{
			Valid:   false,
			Code:    VALIDATION_RATE_LIMIT,
			Message: "rate limit exceeded",
		}
	}

	s.orderCounts[userID] = append(validTimestamps, now)
	return nil
}

func (s *Service) checkPositionLimit(userID uint64, symbol string, side int8, qty int64) *ValidationResult {
	s.mu.RLock()
	limits := s.userLimits[userID]
	s.mu.RUnlock()

	if limits == nil {
		return nil
	}

	currentSize := limits.Positions[symbol]
	newSize := currentSize + qty

	if newSize > s.cfg.MaxPositionSize || newSize < -s.cfg.MaxPositionSize {
		return &ValidationResult{
			Valid:   false,
			Code:    VALIDATION_POSITION_LIMIT,
			Message: "position size would exceed limit",
		}
	}

	return nil
}

func (s *Service) UpdatePosition(userID uint64, symbol string, size int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.userLimits[userID] == nil {
		s.userLimits[userID] = &UserLimits{
			UserID:    userID,
			Positions: make(map[string]int64),
		}
	}

	s.userLimits[userID].Positions[symbol] = size
}

func (s *Service) UpdateDailyPnL(userID uint64, pnl int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.userLimits[userID] == nil {
		s.userLimits[userID] = &UserLimits{
			UserID:    userID,
			Positions: make(map[string]int64),
		}
	}

	s.userLimits[userID].DailyPnL += pnl

	if s.userLimits[userID].DailyPnL < -s.cfg.MaxDailyLoss {
		log.Printf("user %d hit daily loss limit", userID)
	}
}

func (s *Service) GetUserLimits(userID uint64) *UserLimits {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.userLimits[userID]
}

func (s *Service) SetConfig(cfg Config) {
	s.cfg = cfg
}

func (s *Service) Close() {
	s.nats.Close()
}
