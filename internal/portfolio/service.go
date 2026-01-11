package portfolio

import (
	"context"
	"errors"

	"github.com/anomalyco/meta-terminal-go/internal/balance"
	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/events"
	"github.com/anomalyco/meta-terminal-go/internal/messaging"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

var (
	ErrInsufficientBalance = errors.New("insufficient balance")
	ErrLeverageTooHigh     = errors.New("leverage would cause immediate liquidation")
)

type Config struct {
	NATS *messaging.NATS
	Sink events.Sink
}

type Service struct {
	Balances  map[types.UserID]map[string]*types.UserBalance
	Positions map[types.UserID]map[string]*types.Position
	nats      *messaging.NATS
	sink      events.Sink
}

func New(cfg Config) *Service {
	sink := cfg.Sink
	if sink == nil {
		sink = events.NopSink{}
	}
	return &Service{
		Balances:  make(map[types.UserID]map[string]*types.UserBalance),
		Positions: make(map[types.UserID]map[string]*types.Position),
		nats:      cfg.NATS,
		sink:      sink,
	}
}

func (s *Service) GetBalance(userID types.UserID, asset string) *types.UserBalance {
	if b := s.balanceEntry(userID, asset); b != nil {
		return b
	}
	return &types.UserBalance{Asset: asset}
}

func (s *Service) Reserve(userID types.UserID, asset string, amount int64) error {
	userBalances, ok := s.Balances[userID]
	if !ok {
		userBalances = make(map[string]*types.UserBalance)
		s.Balances[userID] = userBalances
	}

	b, ok := userBalances[asset]
	if !ok {
		b = &types.UserBalance{Asset: asset, Available: 0, Locked: 0, Margin: 0}
		userBalances[asset] = b
	}

	if b.Available < amount {
		return ErrInsufficientBalance
	}

	b.Available -= amount
	b.Locked += amount
	return nil
}

func (s *Service) Release(userID types.UserID, asset string, amount int64) {
	b := s.balanceEntry(userID, asset)
	if b == nil {
		return
	}
	b.Locked -= amount
	if b.Locked < 0 {
		b.Locked = 0
	}
	b.Available += amount
}

func (s *Service) ExecuteTrade(trade *types.Trade, taker, maker *types.Order) {
	if trade.Category == constants.CATEGORY_SPOT {
		s.executeSpotTrade(trade, taker, maker)
	} else {
		s.executeLinearTrade(trade, taker, maker)
	}
}

func (s *Service) executeSpotTrade(trade *types.Trade, taker *types.Order, maker *types.Order) {
	baseAsset := balance.GetBaseAsset(trade.Symbol)
	quoteAsset := balance.GetQuoteAsset(trade.Symbol)

	var takerGets, takerPays string
	var makerGets, makerPays string
	var takerGetsQty, takerPaysQty int64
	var makerGetsQty, makerPaysQty int64

	if taker.Side == constants.ORDER_SIDE_BUY {
		takerGets = baseAsset
		takerPays = quoteAsset
		makerGets = quoteAsset
		makerPays = baseAsset
	} else {
		takerGets = quoteAsset
		takerPays = baseAsset
		makerGets = baseAsset
		makerPays = quoteAsset
	}

	takerGetsQty = int64(trade.Quantity)
	takerPaysQty = int64(trade.Price) * int64(trade.Quantity)
	makerGetsQty = takerPaysQty
	makerPaysQty = takerGetsQty

	if b, ok := s.Balances[trade.TakerID][takerGets]; ok {
		b.Available += takerGetsQty
	}
	if b, ok := s.Balances[trade.TakerID][takerPays]; ok {
		b.Available -= takerPaysQty
	}
	if b, ok := s.Balances[trade.MakerID][makerGets]; ok {
		b.Available += makerGetsQty
	}
	if b, ok := s.Balances[trade.MakerID][makerPays]; ok {
		b.Available -= makerPaysQty
	}
}

func (s *Service) executeLinearTrade(trade *types.Trade, taker *types.Order, maker *types.Order) {
	takerPos := s.positionEntry(taker.UserID, trade.Symbol)
	makerPos := s.positionEntry(maker.UserID, trade.Symbol)
	if takerPos == nil {
		takerPos = &types.Position{Symbol: trade.Symbol, Size: 0, Side: -1}
	}
	if makerPos == nil {
		makerPos = &types.Position{Symbol: trade.Symbol, Size: 0, Side: -1}
	}

	takerLeverage := takerPos.Leverage
	if takerLeverage <= 0 {
		takerLeverage = constants.DEFAULT_LEVERAGE
	}
	makerLeverage := makerPos.Leverage
	if makerLeverage <= 0 {
		makerLeverage = constants.DEFAULT_LEVERAGE
	}

	takerQty := (int64(trade.Price) * int64(trade.Quantity)) / int64(takerLeverage)
	makerQty := (int64(trade.Price) * int64(trade.Quantity)) / int64(makerLeverage)

	if b, ok := s.Balances[trade.TakerID]["USDT"]; ok {
		b.Locked -= takerQty
		b.Margin += takerQty
	}
	if b, ok := s.Balances[trade.MakerID]["USDT"]; ok {
		b.Locked -= makerQty
		b.Margin += makerQty
	}

	s.updatePosition(trade.TakerID, trade, taker)
	s.updatePosition(trade.MakerID, trade, maker)
}

func (s *Service) updatePosition(userID types.UserID, trade *types.Trade, order *types.Order) {
	userPositions := s.ensurePositions(userID)
	pos := userPositions[trade.Symbol]
	if pos == nil {
		pos = &types.Position{Symbol: trade.Symbol}
		userPositions[trade.Symbol] = pos
	}

	side := order.Side
	size := int64(trade.Quantity)

	if pos.Size == 0 {
		pos.Size = size * int64(1-side*2)
		pos.EntryPrice = int64(trade.Price)
		if pos.Leverage <= 0 {
			pos.Leverage = constants.DEFAULT_LEVERAGE
		}
		pos.Side = side
	} else if (pos.Size > 0 && side == constants.ORDER_SIDE_BUY) || (pos.Size < 0 && side == constants.ORDER_SIDE_SELL) {
		totalSize := pos.Size + size*int64(1-side*2)
		pos.EntryPrice = (pos.EntryPrice*pos.Size + int64(trade.Price)*size*int64(1-side*2)) / totalSize
		pos.Size = totalSize
	} else {
		closedQty := size
		if remaining := pos.Size - size*int64(1-side*2); remaining < 0 {
			closedQty = (pos.Size + size*int64(1-side*2)) / int64(1-side*2)
		}

		if closedQty > 0 && pos.EntryPrice > 0 {
			rpnl := (int64(trade.Price) - pos.EntryPrice) * closedQty
			if pos.Side == constants.SIDE_SHORT {
				rpnl = (pos.EntryPrice - int64(trade.Price)) * closedQty
			}

			if b, ok := s.Balances[userID]["USDT"]; ok {
				b.Available += rpnl
			}

			event := &types.PositionReducedEvent{
				UserID:       userID,
				Symbol:       trade.Symbol,
				Category:     trade.Category,
				ClosedQty:    closedQty,
				ExitPrice:    int64(trade.Price),
				RPNL:         rpnl,
				PositionSize: pos.Size,
				PositionSide: pos.Side,
				ExecutedAt:   types.NowNano(),
			}
			if s.nats != nil {
				s.nats.PublishGob(context.Background(), messaging.PositionReducedTopic(trade.Symbol), event)
			}
			s.sink.OnPositionReduced(event)
		}

		remaining := pos.Size - size*int64(1-side*2)
		if remaining == 0 {
			pos.Size = 0
			pos.EntryPrice = 0
			pos.Leverage = 0
			pos.Side = -1
		} else {
			pos.Size = remaining
		}
	}
}

func (s *Service) balanceEntry(userID types.UserID, asset string) *types.UserBalance {
	if userBalances := s.Balances[userID]; userBalances != nil {
		return userBalances[asset]
	}
	return nil
}

func (s *Service) positionEntry(userID types.UserID, symbol string) *types.Position {
	if userPositions := s.Positions[userID]; userPositions != nil {
		return userPositions[symbol]
	}
	return nil
}

func (s *Service) ensurePositions(userID types.UserID) map[string]*types.Position {
	userPositions := s.Positions[userID]
	if userPositions == nil {
		userPositions = make(map[string]*types.Position)
		s.Positions[userID] = userPositions
	}
	return userPositions
}

func (s *Service) GetPositions(userID types.UserID) []*types.Position {
	if userPositions, ok := s.Positions[userID]; ok {
		positions := make([]*types.Position, 0, len(userPositions))
		for _, p := range userPositions {
			if p.Size != 0 {
				positions = append(positions, p)
			}
		}
		return positions
	}
	return nil
}

func (s *Service) GetPosition(userID types.UserID, symbol string) *types.Position {
	if userPositions, ok := s.Positions[userID]; ok {
		if pos, ok := userPositions[symbol]; ok {
			return pos
		}
	}
	return &types.Position{Symbol: symbol, Size: 0, Side: -1}
}

func (s *Service) GetLiquidationPrice(pos *types.Position) int64 {
	if pos.Size == 0 || pos.Leverage == 0 {
		return 0
	}

	if pos.Side == constants.ORDER_SIDE_BUY {
		return pos.EntryPrice * int64(100-pos.Leverage*10) / 100
	}
	return pos.EntryPrice * int64(100+pos.Leverage*10) / 100
}

func (s *Service) SetLeverage(userID types.UserID, symbol string, newLeverage int8, currentPrice int64) error {
	userPositions, ok := s.Positions[userID]
	if !ok {
		userPositions = make(map[string]*types.Position)
		s.Positions[userID] = userPositions
	}

	pos, ok := userPositions[symbol]
	if !ok || pos.Size == 0 {
		if newLeverage <= 0 {
			newLeverage = constants.DEFAULT_LEVERAGE
		}
		if newLeverage > 20 {
			newLeverage = 20
		}
		pos = &types.Position{
			Symbol:   symbol,
			Size:     0,
			Side:     -1,
			Leverage: newLeverage,
		}
		userPositions[symbol] = pos
		return nil
	}

	oldLeverage := pos.Leverage
	if oldLeverage <= 0 {
		oldLeverage = constants.DEFAULT_LEVERAGE
	}
	if newLeverage <= 0 {
		newLeverage = constants.DEFAULT_LEVERAGE
	}
	if newLeverage > 20 {
		newLeverage = 20
	}

	if oldLeverage == newLeverage {
		return nil
	}

	marginRelease := -pos.EntryPrice * pos.Size * int64(oldLeverage-newLeverage) / 20

	if newLeverage > oldLeverage {
		if b, ok := s.Balances[userID]["USDT"]; ok {
			if b.Available < marginRelease {
				return ErrInsufficientBalance
			}
			b.Available += marginRelease
			b.Margin -= marginRelease
		}
	} else {
		if b, ok := s.Balances[userID]["USDT"]; ok {
			marginRequired := -marginRelease
			if b.Available < marginRequired {
				return ErrInsufficientBalance
			}
			b.Available -= marginRequired
			b.Margin += marginRequired
		}
	}

	pos.Leverage = newLeverage

	liqPrice := s.calculateLiquidationPrice(pos.EntryPrice, pos.Size, pos.Side, newLeverage)
	if currentPrice > 0 && liqPrice > 0 && currentPrice >= liqPrice {
		pos.Leverage = oldLeverage
		return ErrLeverageTooHigh
	}

	return nil
}

func (s *Service) calculateLiquidationPrice(entryPrice, size int64, side int8, leverage int8) int64 {
	if size == 0 || leverage == 0 {
		return 0
	}
	if side == constants.ORDER_SIDE_BUY {
		return entryPrice * int64(100-leverage*5) / 100
	}
	return entryPrice * int64(100+leverage*5) / 100
}
