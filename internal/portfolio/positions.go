package portfolio

import (
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/maxonlinux/meta-terminal-go/pkg/utils"
	"github.com/robaho/fixed"
)

func (s *Service) GetPosition(userID types.UserID, symbol string) *types.Position {
	pos := s.positionFor(userID, symbol)
	if pos == nil {
		pos = &types.Position{UserID: userID, Symbol: symbol}
		s.ensurePositions(userID)[symbol] = pos
	}
	return pos
}

func (s *Service) SetLeverage(userID types.UserID, symbol string, newLeverage types.Leverage) error {
	pos := s.positionFor(userID, symbol)
	if pos == nil {
		pos = &types.Position{UserID: userID, Symbol: symbol}
		s.ensurePositions(userID)[symbol] = pos
	}

	newLeverage = normalizeLeverage(newLeverage)

	if math.Sign(pos.Size) == 0 {
		pos.Leverage = newLeverage
		return nil
	}

	oldLeverage := normalizeLeverage(pos.Leverage)
	if math.Cmp(oldLeverage, newLeverage) == 0 {
		return nil
	}

	// Resolve quote asset from registry to apply margin changes correctly.
	if s.registry == nil {
		return constants.ErrInstrumentNotFound
	}
	inst := s.registry.GetInstrument(symbol)
	if inst == nil {
		return constants.ErrInstrumentNotFound
	}
	quote := inst.QuoteAsset
	notional := types.Quantity(math.Mul(pos.EntryPrice, absPositionSize(pos.Size)))
	oldMargin := types.Quantity(math.Div(notional, oldLeverage))
	newMargin := types.Quantity(math.Div(notional, newLeverage))
	if err := s.applyMarginDelta(userID, quote, oldMargin, newMargin); err != nil {
		return err
	}

	pos.Leverage = newLeverage
	return nil
}

func (s *Service) updatePosition(userID types.UserID, match *types.Match, order *types.Order) {
	pos := s.positionFor(userID, match.Symbol)
	if pos == nil {
		pos = &types.Position{UserID: userID, Symbol: match.Symbol}
		s.ensurePositions(userID)[match.Symbol] = pos
	}

	signedTrade := sideSignedQty(order.Side, match.Quantity)
	tradeSign := math.Sign(signedTrade)
	posSign := math.Sign(pos.Size)

	if tradeSign == 0 {
		return
	}

	if posSign == 0 {
		pos.Size = signedTrade
		pos.EntryPrice = match.Price
		if math.Sign(pos.Leverage) <= 0 {
			pos.Leverage = types.Leverage(fixed.NewI(int64(constants.DEFAULT_LEVERAGE), 0))
		}
		return
	}

	if posSign == tradeSign {
		newSize := math.Add(pos.Size, signedTrade)
		weighted := math.Add(math.Mul(pos.EntryPrice, pos.Size), math.Mul(match.Price, signedTrade))
		pos.EntryPrice = types.Price(math.Div(weighted, newSize))
		pos.Size = newSize
		return
	}

	remaining := math.Add(pos.Size, signedTrade)
	pos.Size = remaining
	if math.Sign(remaining) == 0 {
		pos.EntryPrice = types.Price(math.Zero)
		pos.Leverage = types.Leverage(math.Zero)
	}

	closedQty := absPositionSize(signedTrade)
	rpnl := realizedPnL(pos.EntryPrice, match.Price, closedQty, pos.Size)
	if math.Sign(rpnl) != 0 {
		// Use registry quote asset for realized PnL balance adjustment.
		if s.registry == nil {
			return
		}
		inst := s.registry.GetInstrument(match.Symbol)
		if inst == nil {
			return
		}
		quote := inst.QuoteAsset
		s.adjustAvailable(userID, quote, rpnl)
		if s.onRealizedPnL != nil {
			// Emit realized PnL for history when a position is reduced.
			timestamp := match.Timestamp
			if timestamp == 0 {
				timestamp = utils.NowNano()
			}
			s.onRealizedPnL(types.RealizedPnL{
				UserID:    userID,
				OrderID:   order.ID,
				Symbol:    match.Symbol,
				Category:  match.Category,
				Side:      order.Side,
				Price:     match.Price,
				Quantity:  closedQty,
				Realized:  rpnl,
				Timestamp: timestamp,
			})
		}
	}
	if s.onReduce != nil {
		s.onReduce(userID, match.Symbol, pos.Size)
	}
}

func (s *Service) positionLeverage(userID types.UserID, symbol string) types.Leverage {
	if pos := s.positionFor(userID, symbol); pos != nil {
		if math.Sign(pos.Leverage) > 0 {
			return pos.Leverage
		}
	}
	return types.Leverage(fixed.NewI(int64(constants.DEFAULT_LEVERAGE), 0))
}

func (s *Service) positionFor(userID types.UserID, symbol string) *types.Position {
	userPositions := s.Positions[userID]
	if userPositions == nil {
		return nil
	}
	return userPositions[symbol]
}

func (s *Service) ensurePositions(userID types.UserID) map[string]*types.Position {
	userPositions := s.Positions[userID]
	if userPositions == nil {
		userPositions = make(map[string]*types.Position)
		s.Positions[userID] = userPositions
	}
	return userPositions
}

func sideSignedQty(side int8, qty types.Quantity) types.Quantity {
	if side == constants.ORDER_SIDE_BUY {
		return qty
	}
	return types.Quantity(math.Neg(qty))
}

func absPositionSize(size types.Quantity) types.Quantity {
	if math.Sign(size) < 0 {
		return types.Quantity(math.Neg(size))
	}
	return size
}

func normalizeLeverage(leverage types.Leverage) types.Leverage {
	if math.Sign(leverage) <= 0 {
		return types.Leverage(fixed.NewI(int64(constants.DEFAULT_LEVERAGE), 0))
	}
	return leverage
}

func (s *Service) applyMarginDelta(userID types.UserID, quote string, oldMargin types.Quantity, newMargin types.Quantity) error {
	delta := types.Quantity(math.Sub(oldMargin, newMargin))
	if math.Sign(delta) == 0 {
		return nil
	}
	if math.Sign(delta) > 0 {
		s.adjustAvailable(userID, quote, delta)
		s.adjustMargin(userID, quote, math.Neg(delta))
		return nil
	}

	required := types.Quantity(math.Neg(delta))
	if math.Lt(s.GetBalance(userID, quote).Available, required) {
		return constants.ErrInsufficientBalance
	}
	s.adjustAvailable(userID, quote, math.Neg(required))
	s.adjustMargin(userID, quote, required)
	return nil
}

func realizedPnL(entryPrice types.Price, tradePrice types.Price, closedQty types.Quantity, remaining types.Quantity) types.Quantity {
	if math.Sign(closedQty) == 0 {
		return types.Quantity(math.Zero)
	}
	priceDiff := math.Sub(tradePrice, entryPrice)
	if math.Sign(remaining) < 0 {
		priceDiff = math.Sub(entryPrice, tradePrice)
	}
	return types.Quantity(math.Mul(priceDiff, closedQty))
}
