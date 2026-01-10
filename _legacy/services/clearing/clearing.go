package clearing

import (
	"context"
	"encoding/binary"
	"log"
	"time"

	"github.com/anomalyco/meta-terminal-go/internal/balance"
	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/messaging"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

type Config struct {
	NATSURL      string
	StreamPrefix string
}

type Service struct {
	nats *messaging.NATS
}

func New(cfg Config) (*Service, error) {
	n, err := messaging.New(messaging.Config{URL: cfg.NATSURL, StreamPrefix: cfg.StreamPrefix})
	if err != nil {
		return nil, err
	}

	return &Service{nats: n}, nil
}

func (s *Service) Start(ctx context.Context) error {
	s.nats.Subscribe(ctx, messaging.SubjectClearingTrade, "clearing-trades", s.handleTrade)
	s.nats.Subscribe(ctx, messaging.SubjectOrderEvent+".>", "clearing-orders", s.handleOrder)
	s.nats.Subscribe(ctx, messaging.SubjectClearingReserve, "clearing-reserve", s.handleReserve)
	s.nats.Subscribe(ctx, messaging.SubjectClearingRelease, "clearing-release", s.handleRelease)
	log.Println("clearing service started")
	return nil
}

func (s *Service) handleReserve(data []byte) {
	if len(data) < 1 {
		return
	}

	msgType := data[0]
	if msgType != 0x01 {
		return
	}

	offset := 1
	if len(data) < offset+8+1+8+1+8 {
		return
	}

	userID := uint64(binary.LittleEndian.Uint64(data[offset:]))
	offset += 8

	symbolLen := int(data[offset])
	offset++
	if len(data) < offset+symbolLen+1+8+1+8 {
		return
	}

	symbol := string(data[offset : offset+symbolLen])
	offset += symbolLen

	category := int8(data[offset])
	offset++
	side := int8(data[offset])
	offset++

	qty := types.Quantity(binary.LittleEndian.Uint64(data[offset:]))
	offset += 8

	price := types.Price(binary.LittleEndian.Uint64(data[offset:]))
	offset += 8

	leverage := int8(data[offset])

	amount, asset := balance.CalculateReserveAmount(symbol, category, side, qty, price, leverage)

	buf := make([]byte, 0, 10+len(asset)+8)
	buf = append(buf, 0x01)
	buf = binary.LittleEndian.AppendUint64(buf, userID)
	buf = append(buf, byte(len(asset)))
	buf = append(buf, asset...)
	buf = binary.LittleEndian.AppendUint64(buf, uint64(amount))

	reply, err := s.nats.RequestReply(context.Background(), "portfolio.reserve", buf, 5*time.Second)
	if err != nil {
		log.Printf("clearing: reserve failed for user=%d asset=%s amount=%d: %v", userID, asset, amount, err)
		return
	}
	if len(reply) == 0 || reply[0] != 0x01 {
		log.Printf("clearing: insufficient balance for user=%d asset=%s amount=%d", userID, asset, amount)
		return
	}
	log.Printf("clearing: reserved user=%d asset=%s amount=%d", userID, asset, amount)
}

func (s *Service) handleRelease(data []byte) {
	if len(data) < 1 {
		return
	}

	msgType := data[0]
	if msgType != 0x02 {
		return
	}

	offset := 1
	if len(data) < offset+8+1+8+1+8 {
		return
	}

	userID := uint64(binary.LittleEndian.Uint64(data[offset:]))
	offset += 8

	symbolLen := int(data[offset])
	offset++
	if len(data) < offset+symbolLen+1+8+1+8 {
		return
	}

	symbol := string(data[offset : offset+symbolLen])
	offset += symbolLen

	category := int8(data[offset])
	offset++
	side := int8(data[offset])
	offset++

	qty := types.Quantity(binary.LittleEndian.Uint64(data[offset:]))
	offset += 8

	price := types.Price(binary.LittleEndian.Uint64(data[offset:]))
	offset += 8

	leverage := int8(data[offset])

	amount, asset := balance.CalculateReserveAmount(symbol, category, side, qty, price, leverage)

	buf := make([]byte, 0, 10+len(asset)+8)
	buf = append(buf, 0x02)
	buf = binary.LittleEndian.AppendUint64(buf, userID)
	buf = append(buf, byte(len(asset)))
	buf = append(buf, asset...)
	buf = binary.LittleEndian.AppendUint64(buf, uint64(amount))

	s.nats.PublishBytes(context.Background(), messaging.SubjectPortfolioRelease, buf)
	log.Printf("clearing: released user=%d asset=%s amount=%d", userID, asset, amount)
}

func (s *Service) handleTrade(data []byte) {
	var trade types.Trade
	if err := messaging.DecodeGob(data, &trade); err != nil {
		log.Printf("clearing: gob decode error: %v", err)
		return
	}
	s.processTrade(&trade)
}

func (s *Service) handleOrder(data []byte) {
	log.Printf("clearing: order event: %s", string(data))
}

func (s *Service) processTrade(trade *types.Trade) {
	if trade.Category == constants.CATEGORY_SPOT {
		s.processSpotTrade(trade)
	} else {
		s.processLinearTrade(trade)
	}
}

func (s *Service) processSpotTrade(trade *types.Trade) {
	takerSide := trade.TakerOrderID % 2

	var takerAmount int64
	var makerAmount int64
	var takerAsset string
	var makerAsset string

	if takerSide == 0 {
		takerAsset = s.getQuoteAsset(trade.Symbol)
		takerAmount = int64(trade.Price) * int64(trade.Quantity)
		makerAsset = s.getBaseAsset(trade.Symbol)
		makerAmount = int64(trade.Quantity)
	} else {
		takerAsset = s.getBaseAsset(trade.Symbol)
		takerAmount = int64(trade.Quantity)
		makerAsset = s.getQuoteAsset(trade.Symbol)
		makerAmount = int64(trade.Price) * int64(trade.Quantity)
	}

	s.publishBalanceUpdate(trade.TakerID, takerAsset, takerAmount, false)
	s.publishBalanceUpdate(trade.MakerID, makerAsset, makerAmount, false)
	s.publishBalanceUpdate(trade.MakerID, takerAsset, takerAmount, true)
	s.publishBalanceUpdate(trade.TakerID, makerAsset, makerAmount, true)

	log.Printf("clearing: spot trade %s %d @ %d taker=%d maker=%d",
		trade.Symbol, trade.Quantity, trade.Price, trade.TakerID, trade.MakerID)
}

func (s *Service) processLinearTrade(trade *types.Trade) {
	takerLeverage := trade.TakerLeverage
	if takerLeverage <= 0 {
		takerLeverage = constants.DEFAULT_LEVERAGE
	}
	makerLeverage := trade.MakerLeverage
	if makerLeverage <= 0 {
		makerLeverage = constants.DEFAULT_LEVERAGE
	}

	takerMargin := int64(trade.Price) * int64(trade.Quantity) / int64(takerLeverage)
	makerMargin := int64(trade.Price) * int64(trade.Quantity) / int64(makerLeverage)

	s.publishMarginUpdate(trade.TakerID, trade.Symbol, takerMargin, false)
	s.publishMarginUpdate(trade.MakerID, trade.Symbol, makerMargin, false)
	s.publishMarginUpdate(trade.TakerID, trade.Symbol, takerMargin, true)
	s.publishMarginUpdate(trade.MakerID, trade.Symbol, makerMargin, true)
	s.publishPositionUpdate(trade.TakerID, trade.Symbol, int64(trade.Quantity), int64(trade.Price), 0, takerLeverage)
	s.publishPositionUpdate(trade.MakerID, trade.Symbol, int64(trade.Quantity), int64(trade.Price), 1, makerLeverage)

	log.Printf("clearing: linear trade %s %d @ %d taker=%d maker=%d takerMargin=%d makerMargin=%d",
		trade.Symbol, trade.Quantity, trade.Price, trade.TakerID, trade.MakerID, takerMargin, makerMargin)
}

func (s *Service) publishBalanceUpdate(userID types.UserID, asset string, amount int64, isAdd bool) {
	subject := "balances." + asset
	action := "DEDUCT"
	if isAdd {
		action = "ADD"
	}
	s.nats.Publish(context.Background(), subject, map[string]interface{}{
		"user_id": userID,
		"asset":   asset,
		"action":  action,
		"amount":  amount,
	})
}

func (s *Service) publishMarginUpdate(userID types.UserID, symbol string, margin int64, isAdd bool) {
	buf := make([]byte, 0, 10+len(symbol)+16)
	buf = append(buf, 0x04)
	buf = binary.LittleEndian.AppendUint64(buf, uint64(userID))
	buf = append(buf, byte(len(symbol)))
	buf = append(buf, symbol...)
	if isAdd {
		buf = binary.LittleEndian.AppendUint64(buf, uint64(margin))
	} else {
		buf = binary.LittleEndian.AppendUint64(buf, uint64(-margin))
	}

	s.nats.PublishBytes(context.Background(), messaging.SubjectPortfolioMargin, buf)
}

func (s *Service) publishPositionUpdate(userID types.UserID, symbol string, size int64, price int64, side int8, leverage int8) {
	buf := make([]byte, 0, 10+len(symbol)+24)
	buf = append(buf, 0x02)
	buf = binary.LittleEndian.AppendUint64(buf, uint64(userID))
	buf = append(buf, byte(len(symbol)))
	buf = append(buf, symbol...)
	buf = binary.LittleEndian.AppendUint64(buf, uint64(size))
	buf = append(buf, byte(side))
	buf = binary.LittleEndian.AppendUint64(buf, uint64(price))
	buf = append(buf, byte(leverage))

	s.nats.PublishBytes(context.Background(), messaging.PositionsEventTopic(symbol), buf)
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

func (s *Service) Close() {
	s.nats.Close()
}
