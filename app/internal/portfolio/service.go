package portfolio

import (
	"fmt"
	"sync"

	"github.com/maxonlinux/meta-terminal-go/internal/registry"
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

type OnPositionReduce func(userID types.UserID, symbol string, size types.Quantity)
type OnBalanceUpdate func(userID types.UserID, asset string, balance *types.Balance)
type OnRealizedPnL func(event types.RealizedPnL)

type Service struct {
	// mu guards balances, positions, and fundings.
	mu            sync.RWMutex
	Balances      map[types.UserID]map[string]*types.Balance
	Positions     map[types.UserID]map[string]*types.Position
	Fundings      map[types.FundingID]*types.FundingRequest
	onReduce      OnPositionReduce
	onBalance     OnBalanceUpdate
	onRealizedPnL OnRealizedPnL
	registry      *registry.Registry
}

func New(onReduce OnPositionReduce, reg *registry.Registry) (*Service, error) {
	if reg == nil {
		return nil, fmt.Errorf("registry is required")
	}
	return &Service{
		Balances:  make(map[types.UserID]map[string]*types.Balance),
		Positions: make(map[types.UserID]map[string]*types.Position),
		Fundings:  make(map[types.FundingID]*types.FundingRequest),
		onReduce:  onReduce,
		registry:  reg,
	}, nil
}

func (s *Service) OnBalanceUpdate(fn OnBalanceUpdate) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Registers a callback for balance updates on portfolio changes.
	s.onBalance = fn
}

func (s *Service) OnRealizedPnL(fn OnRealizedPnL) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Registers a callback for realized PnL events on position reductions.
	s.onRealizedPnL = fn
}

func (s *Service) LoadBalance(balance *types.Balance) {
	if balance == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Balances[balance.UserID] == nil {
		s.Balances[balance.UserID] = make(map[string]*types.Balance)
	}
	s.Balances[balance.UserID][balance.Asset] = balance
}

func (s *Service) LoadPosition(pos *types.Position) {
	if pos == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Positions[pos.UserID] == nil {
		s.Positions[pos.UserID] = make(map[string]*types.Position)
	}
	s.Positions[pos.UserID][pos.Symbol] = pos
}

func (s *Service) ExecuteTrade(match *types.Match) error {
	if match == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if match.Category == constants.CATEGORY_SPOT {
		return s.executeSpotTrade(match)
	}
	return s.executeLinearTrade(match)
}

func (s *Service) executeSpotTrade(match *types.Match) error {
	inst := s.registry.GetInstrument(match.Symbol)
	if inst == nil {
		return constants.ErrInstrumentNotFound
	}
	baseAsset, quoteAsset := inst.BaseAsset, inst.QuoteAsset

	amountBase := match.Quantity
	amountQuote := types.Quantity(math.Mul(match.Price, match.Quantity))

	if match.TakerOrder.Side == constants.ORDER_SIDE_BUY {
		s.applySpotLeg(match.TakerOrder.UserID, baseAsset, quoteAsset, amountBase, amountQuote)
		s.applySpotLeg(match.MakerOrder.UserID, quoteAsset, baseAsset, amountQuote, amountBase)
	} else {
		s.applySpotLeg(match.TakerOrder.UserID, quoteAsset, baseAsset, amountQuote, amountBase)
		s.applySpotLeg(match.MakerOrder.UserID, baseAsset, quoteAsset, amountBase, amountQuote)
	}
	return nil
}

func (s *Service) executeLinearTrade(match *types.Match) error {
	inst := s.registry.GetInstrument(match.Symbol)
	if inst == nil {
		return constants.ErrInstrumentNotFound
	}
	quoteAsset := inst.QuoteAsset
	tradeNotional := types.Quantity(math.Mul(match.Price, match.Quantity))

	takerLeverage := s.positionLeverage(match.TakerOrder.UserID, match.Symbol)
	makerLeverage := s.positionLeverage(match.MakerOrder.UserID, match.Symbol)

	s.applyLinearLeg(match.TakerOrder.UserID, quoteAsset, tradeNotional, takerLeverage)
	s.applyLinearLeg(match.MakerOrder.UserID, quoteAsset, tradeNotional, makerLeverage)

	if err := s.updatePosition(match.TakerOrder.UserID, match, match.TakerOrder); err != nil {
		return err
	}
	if err := s.updatePosition(match.MakerOrder.UserID, match, match.MakerOrder); err != nil {
		return err
	}
	return nil
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
	s.mu.RLock()
	defer s.mu.RUnlock()
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
	s.mu.RLock()
	defer s.mu.RUnlock()
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
