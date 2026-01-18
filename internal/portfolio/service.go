package portfolio

import (
	"github.com/maxonlinux/meta-terminal-go/internal/balance"
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

// PositionReduceHandler notifies the engine about reduced positions.
type PositionReduceHandler func(userID types.UserID, symbol string, size types.Quantity)

// Service stores balances and positions for users in memory.
type Service struct {
	Balances  map[types.UserID]map[string]*types.Balance
	Positions map[types.UserID]map[string]*types.Position
	Fundings  map[types.FundingID]*types.FundingRequest
	onReduce  PositionReduceHandler
}

// New creates a portfolio service with empty state.
func New(onReduce PositionReduceHandler) *Service {
	return &Service{
		Balances:  make(map[types.UserID]map[string]*types.Balance),
		Positions: make(map[types.UserID]map[string]*types.Position),
		Fundings:  make(map[types.FundingID]*types.FundingRequest),
		onReduce:  onReduce,
	}
}

// ExecuteTrade applies a match to balances and positions.
func (s *Service) ExecuteTrade(match *types.Match) {
	if match.Category == constants.CATEGORY_SPOT {
		s.executeSpotTrade(match)
		return
	}
	s.executeLinearTrade(match)
}

// executeSpotTrade transfers base/quote assets between counterparties.
func (s *Service) executeSpotTrade(match *types.Match) {
	baseAsset := balance.GetBaseAsset(match.Symbol)
	quoteAsset := balance.GetQuoteAsset(match.Symbol)

	takerGets, takerPays := baseAsset, quoteAsset
	makerGets, makerPays := quoteAsset, baseAsset
	if match.TakerOrder.Side == constants.ORDER_SIDE_SELL {
		takerGets, takerPays = quoteAsset, baseAsset
		makerGets, makerPays = baseAsset, quoteAsset
	}

	amountBase := match.Quantity
	amountQuote := types.Quantity(math.Mul(match.Price, match.Quantity))

	s.applySpotLeg(match.TakerOrder.UserID, takerGets, takerPays, amountBase, amountQuote)
	s.applySpotLeg(match.MakerOrder.UserID, makerGets, makerPays, amountQuote, amountBase)
}

// executeLinearTrade updates margin buckets and positions for LINEAR trades.
func (s *Service) executeLinearTrade(match *types.Match) {
	quoteAsset := balance.GetQuoteAsset(match.Symbol)
	tradeNotional := types.Quantity(math.Mul(match.Price, match.Quantity))

	takerLeverage := s.positionLeverage(match.TakerOrder.UserID, match.Symbol)
	makerLeverage := s.positionLeverage(match.MakerOrder.UserID, match.Symbol)

	s.applyLinearLeg(match.TakerOrder.UserID, quoteAsset, tradeNotional, takerLeverage)
	s.applyLinearLeg(match.MakerOrder.UserID, quoteAsset, tradeNotional, makerLeverage)

	s.updatePosition(match.TakerOrder.UserID, match, match.TakerOrder)
	s.updatePosition(match.MakerOrder.UserID, match, match.MakerOrder)
}

func (s *Service) applySpotLeg(userID types.UserID, getsAsset string, paysAsset string, getsQty types.Quantity, paysQty types.Quantity) {
	s.adjustAvailable(userID, getsAsset, getsQty)
	s.adjustAvailable(userID, paysAsset, math.Neg(paysQty))
}

func (s *Service) applyLinearLeg(userID types.UserID, quoteAsset string, tradeNotional types.Quantity, leverage types.Leverage) {
	margin := types.Quantity(math.Div(tradeNotional, leverage))
	s.adjustLocked(userID, quoteAsset, math.Neg(margin))
	s.adjustMargin(userID, quoteAsset, margin)
}
