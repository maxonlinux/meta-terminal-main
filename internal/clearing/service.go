package clearing

import (
	"sync"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

type Portfolio interface {
	Reserve(userID types.UserID, asset string, amount int64) error
	Release(userID types.UserID, asset string, amount int64)
	ExecuteTrade(trade *types.Trade, taker, maker *types.Order)
	GetPositions(userID types.UserID) []*types.Position
	GetPosition(userID types.UserID, symbol string) *types.Position
}

type Service struct {
	portfolio Portfolio
	mu        sync.RWMutex
}

func New(portfolio Portfolio) *Service {
	return &Service{portfolio: portfolio}
}

func (s *Service) Reserve(userID types.UserID, symbol string, category int8, side int8, qty types.Quantity, price types.Price) error {
	amount, asset := s.calculateReserveAmount(userID, symbol, category, side, qty, price)
	return s.portfolio.Reserve(userID, asset, amount)
}

func (s *Service) Release(userID types.UserID, symbol string, category int8, side int8, qty types.Quantity, price types.Price) {
	amount, asset := s.calculateReserveAmount(userID, symbol, category, side, qty, price)
	s.portfolio.Release(userID, asset, amount)
}

func (s *Service) calculateReserveAmount(userID types.UserID, symbol string, category int8, side int8, qty types.Quantity, price types.Price) (int64, string) {
	if category == constants.CATEGORY_SPOT {
		if side == constants.ORDER_SIDE_BUY {
			return int64(qty) * int64(price), "USDT"
		}
		return int64(qty), symbol
	}

	pos := s.portfolio.GetPosition(userID, symbol)
	leverage := pos.Leverage
	if leverage <= 0 {
		leverage = constants.DEFAULT_LEVERAGE
	}
	return (int64(qty) * int64(price)) / int64(leverage), "USDT"
}

func (s *Service) ExecuteTrade(trade *types.Trade, taker *types.Order, maker *types.Order) {
	if trade.Category == constants.CATEGORY_SPOT {
		s.executeSpotTrade(trade, taker, maker)
	} else {
		s.executeLinearTrade(trade, taker, maker)
	}
}

func (s *Service) executeSpotTrade(trade *types.Trade, taker *types.Order, maker *types.Order) {
	if trade.TakerOrderID == taker.ID {
		s.processSpotTaker(taker, maker, trade)
	} else {
		s.processSpotTaker(maker, taker, trade)
	}
}

func (s *Service) processSpotTaker(taker *types.Order, maker *types.Order, trade *types.Trade) {
	takerAsset := taker.Symbol
	makerAsset := "USDT"

	takerAmount := int64(trade.Quantity)
	makerAmount := int64(trade.Price) * int64(trade.Quantity)

	if taker.Side == constants.ORDER_SIDE_SELL {
		takerAsset = "USDT"
		makerAsset = taker.Symbol
		takerAmount = int64(trade.Price) * int64(trade.Quantity)
		makerAmount = int64(trade.Quantity)
	}

	s.portfolio.Release(taker.UserID, takerAsset, takerAmount)
	s.portfolio.Release(maker.UserID, makerAsset, makerAmount)

	s.portfolio.Reserve(taker.UserID, makerAsset, makerAmount)
	s.portfolio.Reserve(maker.UserID, takerAsset, takerAmount)
}

func (s *Service) executeLinearTrade(trade *types.Trade, taker *types.Order, maker *types.Order) {
	takerPos := s.portfolio.GetPosition(taker.UserID, trade.Symbol)
	makerPos := s.portfolio.GetPosition(maker.UserID, trade.Symbol)

	takerLeverage := takerPos.Leverage
	if takerLeverage <= 0 {
		takerLeverage = constants.DEFAULT_LEVERAGE
	}
	makerLeverage := makerPos.Leverage
	if makerLeverage <= 0 {
		makerLeverage = constants.DEFAULT_LEVERAGE
	}

	takerAmount := (int64(trade.Price) * int64(trade.Quantity)) / int64(takerLeverage)
	makerAmount := (int64(trade.Price) * int64(trade.Quantity)) / int64(makerLeverage)

	s.portfolio.Release(taker.UserID, "USDT", takerAmount)
	s.portfolio.Release(maker.UserID, "USDT", makerAmount)

	s.portfolio.Reserve(taker.UserID, "USDT", takerAmount)
	s.portfolio.Reserve(maker.UserID, "USDT", makerAmount)

	s.portfolio.ExecuteTrade(trade, taker, maker)
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
