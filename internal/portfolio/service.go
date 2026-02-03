package portfolio

import (
	"github.com/maxonlinux/meta-terminal-go/internal/registry"
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

type OnPositionReduce func(userID types.UserID, symbol string, size types.Quantity)
type OnBalanceUpdate func(userID types.UserID, asset string, balance *types.Balance)

type Service struct {
	Balances  map[types.UserID]map[string]*types.Balance
	Positions map[types.UserID]map[string]*types.Position
	Fundings  map[types.FundingID]*types.FundingRequest
	onReduce  OnPositionReduce
	onBalance OnBalanceUpdate
	registry  *registry.Registry
}

func New(onReduce OnPositionReduce, reg *registry.Registry) *Service {
	s := &Service{
		Balances:  make(map[types.UserID]map[string]*types.Balance),
		Positions: make(map[types.UserID]map[string]*types.Position),
		Fundings:  make(map[types.FundingID]*types.FundingRequest),
		onReduce:  onReduce,
		registry:  reg,
	}
	return s
}

func (s *Service) SetBalanceUpdate(fn OnBalanceUpdate) {
	s.onBalance = fn
}

func (s *Service) LoadBalance(balance *types.Balance) {
	if balance == nil {
		return
	}
	if s.Balances[balance.UserID] == nil {
		s.Balances[balance.UserID] = make(map[string]*types.Balance)
	}
	s.Balances[balance.UserID][balance.Asset] = balance
}

func (s *Service) LoadPosition(pos *types.Position) {
	if pos == nil {
		return
	}
	if s.Positions[pos.UserID] == nil {
		s.Positions[pos.UserID] = make(map[string]*types.Position)
	}
	s.Positions[pos.UserID][pos.Symbol] = pos
}

func (s *Service) GetInstrument(symbol string) *types.Instrument {
	if s.registry != nil {
		return s.registry.GetInstrument(symbol)
	}
	return nil
}

func (s *Service) ExecuteTrade(match *types.Match) {
	if match.Category == constants.CATEGORY_SPOT {
		s.executeSpotTrade(match)
		return
	}
	s.executeLinearTrade(match)
}

func (s *Service) executeSpotTrade(match *types.Match) {
	inst := s.GetInstrument(match.Symbol)
	baseAsset, quoteAsset := inst.BaseAsset, inst.QuoteAsset

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

func (s *Service) executeLinearTrade(match *types.Match) {
	inst := s.GetInstrument(match.Symbol)
	quoteAsset := inst.QuoteAsset
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
	s.adjustLocked(userID, paysAsset, math.Neg(paysQty))
}

func (s *Service) applyLinearLeg(userID types.UserID, quoteAsset string, tradeNotional types.Quantity, leverage types.Leverage) {
	margin := types.Quantity(math.Div(tradeNotional, leverage))
	s.adjustLocked(userID, quoteAsset, math.Neg(margin))
	s.adjustMargin(userID, quoteAsset, margin)
}

func (s *Service) GetPositions(userID types.UserID) []*types.Position {
	positions := s.Positions[userID]
	if positions == nil {
		return nil
	}

	result := make([]*types.Position, 0, len(positions))
	for _, pos := range positions {
		result = append(result, pos)
	}
	return result
}

func (s *Service) GetBalances(userID types.UserID) []*types.Balance {
	balances := s.Balances[userID]
	if balances == nil {
		return nil
	}

	result := make([]*types.Balance, 0, len(balances))
	for _, bal := range balances {
		result = append(result, bal)
	}
	return result
}
