package portfolio

import (
	"github.com/maxonlinux/meta-terminal-go/internal/balance"
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

// GetPosition returns the stored position for the symbol.
func (s *Service) GetPosition(userID types.UserID, symbol string) *types.Position {
	pos := s.positionFor(userID, symbol)
	if pos == nil {
		pos = &types.Position{UserID: userID, Symbol: symbol}
		s.ensurePositions(userID)[symbol] = pos
	}
	return pos
}

// SetLeverage updates the leverage for a symbol and adjusts margin buckets.
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

	quote := balance.GetQuoteAsset(symbol)
	notional := types.Quantity(math.Mul(pos.EntryPrice, absPositionSize(pos.Size)))
	oldMargin := types.Quantity(math.Div(notional, oldLeverage))
	newMargin := types.Quantity(math.Div(notional, newLeverage))
	if err := s.applyMarginDelta(userID, quote, oldMargin, newMargin); err != nil {
		return err
	}

	pos.Leverage = newLeverage
	return nil
}

// updatePosition applies trade size and average price changes.
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

	// New position: set size and entry price using the trade direction.
	if posSign == 0 {
		pos.Size = signedTrade
		pos.EntryPrice = match.Price
		if math.Sign(pos.Leverage) <= 0 {
			pos.Leverage = balance.DefaultLeverage()
		}
		return
	}

	// Increase: trade matches the current position direction.
	if posSign == tradeSign {
		newSize := math.Add(pos.Size, signedTrade)
		weighted := math.Add(math.Mul(pos.EntryPrice, pos.Size), math.Mul(match.Price, signedTrade))
		pos.EntryPrice = types.Price(math.Div(weighted, newSize))
		pos.Size = newSize
		return
	}

	// Reduce/flip: trade direction opposes the current position.
	remaining := math.Add(pos.Size, signedTrade)
	pos.Size = remaining
	if math.Sign(remaining) == 0 {
		pos.EntryPrice = types.Price(math.Zero)
		pos.Leverage = types.Leverage(math.Zero)
	}

	closedQty := absPositionSize(signedTrade)
	rpnl := realizedPnL(pos.EntryPrice, match.Price, closedQty, pos.Size)
	if math.Sign(rpnl) != 0 {
		quote := balance.GetQuoteAsset(match.Symbol)
		s.adjustAvailable(userID, quote, rpnl)
	}
	// Notify reduce-only index about updated position size.
	if s.onReduce != nil {
		s.onReduce(userID, match.Symbol, pos.Size)
	}
}

// positionLeverage returns the active leverage for a symbol.
func (s *Service) positionLeverage(userID types.UserID, symbol string) types.Leverage {
	if pos := s.positionFor(userID, symbol); pos != nil {
		if math.Sign(pos.Leverage) > 0 {
			return pos.Leverage
		}
	}
	return balance.DefaultLeverage()
}

func (s *Service) positionFor(userID types.UserID, symbol string) *types.Position {
	if userPositions := s.Positions[userID]; userPositions != nil {
		return userPositions[symbol]
	}
	return nil
}

// ensurePositions returns or creates the positions map for a user.
func (s *Service) ensurePositions(userID types.UserID) map[string]*types.Position {
	userPositions := s.Positions[userID]
	if userPositions == nil {
		userPositions = make(map[string]*types.Position)
		s.Positions[userID] = userPositions
	}
	return userPositions
}

// sideSignedQty returns order quantity signed by order side.
func sideSignedQty(side int8, qty types.Quantity) types.Quantity {
	if side == constants.ORDER_SIDE_BUY {
		return qty
	}
	return types.Quantity(math.Neg(qty))
}

// absPositionSize returns the absolute position size.
func absPositionSize(size types.Quantity) types.Quantity {
	if math.Sign(size) < 0 {
		return types.Quantity(math.Neg(size))
	}
	return size
}

func normalizeLeverage(leverage types.Leverage) types.Leverage {
	if math.Sign(leverage) <= 0 {
		return balance.DefaultLeverage()
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

// realizedPnL calculates realized PnL for the closing quantity.
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
