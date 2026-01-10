package portfolio

import (
	"context"
	"errors"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/messaging"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

var (
	ErrInsufficientBalance = errors.New("insufficient balance")
	ErrLeverageTooHigh     = errors.New("leverage would cause immediate liquidation")
)

func getBaseAsset(symbol string) string {
	quotes := []string{"USDT", "USD", "USDC", "BUSD"}
	for _, q := range quotes {
		if len(symbol) > len(q) && symbol[len(symbol)-len(q):] == q {
			return symbol[:len(symbol)-len(q)]
		}
	}
	return symbol
}

func getQuoteAsset(symbol string) string {
	quotes := []string{"USDT", "USD", "USDC", "BUSD"}
	for _, q := range quotes {
		if len(symbol) > len(q) && symbol[len(symbol)-len(q):] == q {
			return q
		}
	}
	return "USD"
}

type Config struct {
	NATS *messaging.NATS
}

type Service struct {
	Balances  map[types.UserID]map[string]*types.UserBalance
	Positions map[types.UserID]map[string]*types.Position
	nats      *messaging.NATS
}

func New(cfg Config) *Service {
	return &Service{
		Balances:  make(map[types.UserID]map[string]*types.UserBalance),
		Positions: make(map[types.UserID]map[string]*types.Position),
		nats:      cfg.NATS,
	}
}

func (s *Service) GetBalance(userID types.UserID, asset string) *types.UserBalance {
	if userBalances, ok := s.Balances[userID]; ok {
		if b, ok := userBalances[asset]; ok {
			return b
		}
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
	if userBalances, ok := s.Balances[userID]; ok {
		if b, ok := userBalances[asset]; ok {
			b.Locked -= amount
			if b.Locked < 0 {
				b.Locked = 0
			}
			b.Available += amount
		}
	}
}

func (s *Service) ExecuteTrade(trade *types.Trade, taker, maker *types.Order) {
	if trade.Category == constants.CATEGORY_SPOT {
		s.executeSpotTrade(trade, taker, maker)
	} else {
		s.executeLinearTrade(trade, taker, maker)
	}
}

func (s *Service) executeSpotTrade(trade *types.Trade, taker *types.Order, maker *types.Order) {
	baseAsset := getBaseAsset(trade.Symbol)
	quoteAsset := getQuoteAsset(trade.Symbol)

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
	var takerPos, makerPos *types.Position

	if userPositions, ok := s.Positions[taker.UserID]; ok {
		if pos, ok := userPositions[trade.Symbol]; ok {
			takerPos = pos
		}
	}
	if userPositions, ok := s.Positions[maker.UserID]; ok {
		if pos, ok := userPositions[trade.Symbol]; ok {
			makerPos = pos
		}
	}
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
	userPositions, ok := s.Positions[userID]
	if !ok {
		userPositions = make(map[string]*types.Position)
		s.Positions[userID] = userPositions
	}

	key := trade.Symbol
	pos, ok := userPositions[key]
	if !ok {
		pos = &types.Position{Symbol: trade.Symbol}
		userPositions[key] = pos
	}

	side := order.Side
	size := int64(trade.Quantity)

	if pos.Size == 0 {
		pos.Size = size * int64(1-side*2)
		pos.EntryPrice = int64(trade.Price)
		pos.Leverage = constants.DEFAULT_LEVERAGE
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

			s.publishPositionReduced(userID, trade.Symbol, trade.Category, closedQty, int64(trade.Price), rpnl, pos)
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

func (s *Service) publishPositionReduced(userID types.UserID, symbol string, category int8, closedQty, exitPrice, rpnl int64, pos *types.Position) {
	if s.nats == nil {
		return
	}

	event := &types.PositionReducedEvent{
		UserID:       userID,
		Symbol:       symbol,
		Category:     category,
		ClosedQty:    closedQty,
		ExitPrice:    exitPrice,
		RPNL:         rpnl,
		PositionSize: pos.Size,
		PositionSide: pos.Side,
		ExecutedAt:   types.NowNano(),
	}
	s.nats.PublishGob(context.Background(), messaging.PositionReducedTopic(symbol), event)
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

func (s *Service) CalculateRPNL(userID types.UserID, symbol string, exitPrice int64, size int64) int64 {
	userPositions, ok := s.Positions[userID]
	if !ok {
		return 0
	}

	pos, ok := userPositions[symbol]
	if !ok || pos.Size == 0 {
		return 0
	}

	if pos.Side == constants.ORDER_SIDE_BUY {
		return (exitPrice - pos.EntryPrice) * size
	}
	return (pos.EntryPrice - exitPrice) * size
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
