package clearing

import (
	"sync"

	"github.com/anomalyco/meta-terminal-go/internal/balance"
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
		base := balance.GetBaseAsset(symbol)
		quote := balance.GetQuoteAsset(symbol)
		if side == constants.ORDER_SIDE_BUY {
			return int64(qty) * int64(price), quote
		}
		return int64(qty), base
	}

	pos := s.portfolio.GetPosition(userID, symbol)
	leverage := pos.Leverage
	if leverage <= 0 {
		leverage = constants.DEFAULT_LEVERAGE
	}
	quote := balance.GetQuoteAsset(symbol)
	return (int64(qty) * int64(price)) / int64(leverage), quote
}

func (s *Service) ExecuteTrade(trade *types.Trade, taker *types.Order, maker *types.Order) {
	if trade.Category == constants.CATEGORY_SPOT {
		s.executeSpotTrade(trade, taker, maker)
	} else {
		s.executeLinearTrade(trade, taker, maker)
	}
}

func (s *Service) executeSpotTrade(trade *types.Trade, taker *types.Order, maker *types.Order) {
	base := balance.GetBaseAsset(trade.Symbol)
	quote := balance.GetQuoteAsset(trade.Symbol)
	baseQty := int64(trade.Quantity)
	quoteQty := int64(trade.Price) * int64(trade.Quantity)

	if taker.Side == constants.ORDER_SIDE_BUY {
		s.portfolio.Release(taker.UserID, quote, quoteQty)
		s.portfolio.Release(maker.UserID, base, baseQty)
	} else {
		s.portfolio.Release(taker.UserID, base, baseQty)
		s.portfolio.Release(maker.UserID, quote, quoteQty)
	}

	s.portfolio.ExecuteTrade(trade, taker, maker)
}

func (s *Service) executeLinearTrade(trade *types.Trade, taker *types.Order, maker *types.Order) {
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
