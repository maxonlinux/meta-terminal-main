package portfolio

import (
	"context"
	"encoding/binary"
	"log"
	"sync"
	"time"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/messaging"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

type Config struct {
	NATSURL      string
	StreamPrefix string
}

type Balance struct {
	Available int64
	Locked    int64
	Margin    int64
}

type Position struct {
	Symbol     string
	Size       int64
	Side       int8
	EntryPrice int64
	Leverage   int8
}

type ReserveRequest struct {
	UserID   uint64
	Symbol   string
	Category int8
	Side     int8
	Qty      int64
	Price    int64
	Leverage int8
}

type Service struct {
	nats      *messaging.NATS
	balances  map[uint64]map[string]Balance
	positions map[uint64]map[string]Position
	mu        sync.RWMutex
}

func New(cfg Config) (*Service, error) {
	n, err := messaging.New(messaging.Config{URL: cfg.NATSURL, StreamPrefix: cfg.StreamPrefix})
	if err != nil {
		return nil, err
	}

	return &Service{
		nats:      n,
		balances:  make(map[uint64]map[string]Balance),
		positions: make(map[uint64]map[string]Position),
	}, nil
}

func (s *Service) Start(ctx context.Context) error {
	go s.handleReserveRequests(ctx)
	s.nats.Subscribe(ctx, messaging.SubjectPortfolioMargin, "portfolio-margin", s.handleMargin)
	s.nats.Subscribe(ctx, messaging.SubjectPositionsEvent+".>", "portfolio-positions", s.handlePosition)
	log.Println("portfolio started")
	return nil
}

func (s *Service) handleReserveRequests(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			msg, err := s.nats.RequestReply(ctx, "portfolio.reserve", []byte{}, 5*time.Second)
			if err != nil {
				time.Sleep(10 * time.Millisecond)
				continue
			}
			s.processReserveRequest(msg)
		}
	}
}

func (s *Service) processReserveRequest(data []byte) {
	if len(data) < 10 {
		s.nats.PublishBytes(context.Background(), "portfolio.reserve.reply", []byte{0x00})
		return
	}

	offset := 1
	userID := uint64(binary.LittleEndian.Uint64(data[offset:]))
	offset += 8
	category := int8(data[offset])
	offset++
	symbolLen := int(data[offset])
	offset++
	if len(data) < offset+symbolLen+8 {
		s.nats.PublishBytes(context.Background(), "portfolio.reserve.reply", []byte{0x00})
		return
	}
	symbol := string(data[offset : offset+symbolLen])
	offset += symbolLen
	side := int8(data[offset])
	offset++
	qty := int64(binary.LittleEndian.Uint64(data[offset:]))
	offset += 8
	price := int64(binary.LittleEndian.Uint64(data[offset:]))
	offset += 8
	leverage := int8(data[offset])

	amount, asset := s.calculateAmount(userID, symbol, category, side, qty, price, leverage)

	s.mu.Lock()
	if s.balances[userID] == nil {
		s.balances[userID] = make(map[string]Balance)
	}

	b := s.balances[userID][asset]
	if b.Available < amount {
		s.mu.Unlock()
		s.nats.PublishBytes(context.Background(), "portfolio.reserve.reply", []byte{0x00})
		log.Printf("portfolio: reserve failed user=%d asset=%s avail=%d req=%d",
			userID, asset, b.Available, amount)
		return
	}

	b.Available -= amount
	b.Locked += amount
	s.balances[userID][asset] = b
	s.mu.Unlock()

	s.nats.PublishBytes(context.Background(), "portfolio.reserve.reply", []byte{0x01})
	log.Printf("portfolio: reserved user=%d asset=%s amount=%d avail=%d locked=%d",
		userID, asset, amount, b.Available, b.Locked)
}

func (s *Service) calculateAmount(userID uint64, symbol string, category int8, side int8, qty int64, price int64, leverage int8) (int64, string) {
	if category == constants.CATEGORY_SPOT {
		if side == constants.ORDER_SIDE_BUY {
			return qty * price, s.getQuoteAsset(symbol)
		}
		return qty, s.getBaseAsset(symbol)
	}
	return (qty * price) / int64(leverage), s.getQuoteAsset(symbol)
}

func (s *Service) handleMargin(data []byte) {
	if len(data) < 10 {
		return
	}

	offset := 1
	userID := uint64(binary.LittleEndian.Uint64(data[offset:]))
	offset += 8
	symbolLen := int(data[offset])
	offset++
	if len(data) < offset+symbolLen+8 {
		return
	}
	symbol := string(data[offset : offset+symbolLen])
	offset += symbolLen
	margin := int64(binary.LittleEndian.Uint64(data[offset : offset+8]))

	s.mu.Lock()
	defer s.mu.Unlock()

	asset := s.getQuoteAsset(symbol)
	b := s.balances[userID][asset]
	b.Locked -= margin
	b.Margin += margin
	s.balances[userID][asset] = b

	log.Printf("portfolio: margin user=%d symbol=%s margin=%d locked=%d margin_bal=%d",
		userID, symbol, margin, b.Locked, b.Margin)
}

func (s *Service) handlePosition(data []byte) {
	if len(data) < 10 {
		return
	}

	offset := 1
	userID := uint64(binary.LittleEndian.Uint64(data[offset:]))
	offset += 8
	symbolLen := int(data[offset])
	offset++
	if len(data) < offset+symbolLen+16 {
		return
	}
	symbol := string(data[offset : offset+symbolLen])
	offset += symbolLen
	size := int64(binary.LittleEndian.Uint64(data[offset:]))
	offset += 8
	side := int8(data[offset])
	offset++
	price := int64(binary.LittleEndian.Uint64(data[offset:]))
	offset += 8
	leverage := int8(data[offset])

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.positions[userID] == nil {
		s.positions[userID] = make(map[string]Position)
	}

	pos := s.positions[userID][symbol]
	if pos.Size == 0 {
		effectiveLeverage := leverage
		if effectiveLeverage <= 0 {
			effectiveLeverage = constants.DEFAULT_LEVERAGE
		}
		pos = Position{
			Symbol:     symbol,
			Size:       size,
			Side:       side,
			EntryPrice: price,
			Leverage:   effectiveLeverage,
		}
	} else if pos.Side == side {
		totalSize := pos.Size + size
		pos.EntryPrice = (pos.EntryPrice*pos.Size + price*size) / totalSize
		pos.Size = totalSize
	} else {
		pos.Size -= size
		if pos.Size < 0 {
			pos.Side = -pos.Side
			pos.Size = -pos.Size
		}
		if pos.Size == 0 {
			delete(s.positions[userID], symbol)
			return
		}
	}
	s.positions[userID][symbol] = pos

	log.Printf("portfolio: position user=%d symbol=%s size=%d side=%d entry=%d",
		userID, symbol, pos.Size, pos.Side, pos.EntryPrice)
}

func (s *Service) Release(ctx context.Context, userID uint64, symbol string, category int8, side int8, qty types.Quantity, price types.Price, leverage int8) error {
	amount, asset := s.calculateAmount(userID, symbol, category, side, int64(qty), int64(price), leverage)

	s.mu.Lock()
	defer s.mu.Unlock()

	b := s.balances[userID][asset]
	b.Available += amount
	b.Locked -= amount
	s.balances[userID][asset] = b

	log.Printf("portfolio: released user=%d asset=%s amount=%d avail=%d locked=%d",
		userID, asset, amount, b.Available, b.Locked)

	return nil
}

func (s *Service) getQuoteAsset(symbol string) string {
	quotes := []string{"USDT", "USD", "USDC", "BUSD"}
	for _, q := range quotes {
		if len(symbol) > len(q) && symbol[len(symbol)-len(q):] == q {
			return q
		}
	}
	return "USD"
}

func (s *Service) getBaseAsset(symbol string) string {
	quotes := []string{"USDT", "USD", "USDC", "BUSD"}
	for _, q := range quotes {
		if len(symbol) > len(q) && symbol[len(symbol)-len(q):] == q {
			return symbol[:len(symbol)-len(q)]
		}
	}
	return symbol
}

func (s *Service) GetBalance(userID uint64, asset string) Balance {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if b, ok := s.balances[userID][asset]; ok {
		return b
	}
	return Balance{}
}

func (s *Service) GetPosition(userID uint64, symbol string) Position {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if pos, ok := s.positions[userID][symbol]; ok {
		return pos
	}
	return Position{Symbol: symbol}
}

func (s *Service) Close() {
	s.nats.Close()
}
