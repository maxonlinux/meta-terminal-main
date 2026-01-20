package clearing

import (
	"github.com/maxonlinux/meta-terminal-go/internal/balance"
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/robaho/fixed"
)

// Portfolio defines the balance/position operations required by clearing.
type Portfolio interface {
	Reserve(userID types.UserID, asset string, amount types.Quantity) error
	Release(userID types.UserID, asset string, amount types.Quantity)
	ExecuteTrade(match *types.Match)
	GetPosition(userID types.UserID, symbol string) *types.Position
}

// Service coordinates reservation and execution flows.
type Service struct {
	portfolio Portfolio
}

// New creates a clearing component bound to the portfolio implementation.
func New(portfolio Portfolio) *Service {
	return &Service{portfolio: portfolio}
}

// Reserve reserves funds required for an order.
func (s *Service) Reserve(userID types.UserID, symbol string, category int8, side int8, qty types.Quantity, price types.Price) error {
	amount, asset := CalculateReserveAmount(symbol, category, side, qty, price, s.leverageFor(userID, symbol))
	return s.portfolio.Reserve(userID, asset, amount)
}

// Release releases reserved funds when an order is canceled.
func (s *Service) Release(userID types.UserID, symbol string, category int8, side int8, qty types.Quantity, price types.Price) {
	amount, asset := CalculateReserveAmount(symbol, category, side, qty, price, s.leverageFor(userID, symbol))
	s.portfolio.Release(userID, asset, amount)
}

// ExecuteTrade delegates trade settlement to the portfolio service.
func (s *Service) ExecuteTrade(match *types.Match) {
	if match.Category == constants.CATEGORY_SPOT {
		base := balance.GetBaseAsset(match.Symbol)
		quote := balance.GetQuoteAsset(match.Symbol)
		baseQty := match.Quantity
		quoteQty := types.Quantity(math.Mul(match.Price, match.Quantity))

		if match.TakerOrder.Side == constants.ORDER_SIDE_BUY {
			s.portfolio.Release(match.TakerOrder.UserID, quote, quoteQty)
			s.portfolio.Release(match.MakerOrder.UserID, base, baseQty)
		} else {
			s.portfolio.Release(match.TakerOrder.UserID, base, baseQty)
			s.portfolio.Release(match.MakerOrder.UserID, quote, quoteQty)
		}
	}

	s.portfolio.ExecuteTrade(match)
}

// leverageFor returns the leverage for a user position or default leverage.
func (s *Service) leverageFor(userID types.UserID, symbol string) types.Leverage {
	pos := s.portfolio.GetPosition(userID, symbol)
	if math.Sign(pos.Leverage) > 0 {
		return pos.Leverage
	}
	return types.Leverage(fixed.NewI(int64(constants.DEFAULT_LEVERAGE), 0))
}

// CalculateReserveAmount computes the reservation amount for a new order.
func CalculateReserveAmount(symbol string, category int8, side int8, qty types.Quantity, price types.Price, leverage types.Leverage) (types.Quantity, string) {
	if category == constants.CATEGORY_SPOT {
		if side == constants.ORDER_SIDE_BUY {
			return types.Quantity(math.Mul(qty, price)), balance.GetQuoteAsset(symbol)
		}
		return qty, balance.GetBaseAsset(symbol)
	}

	effective := leverage
	if math.Sign(effective) <= 0 {
		effective = types.Leverage(fixed.NewI(int64(constants.DEFAULT_LEVERAGE), 0))
	}
	reserve := math.MulDiv(qty, price, effective)
	return types.Quantity(reserve), balance.GetQuoteAsset(symbol)
}

// LiquidationPrice calculates the liquidation price based on leverage and side.
func LiquidationPrice(entryPrice types.Price, leverage types.Leverage, size types.Quantity) types.Price {
	if math.Sign(entryPrice) <= 0 || math.Sign(leverage) <= 0 {
		return types.Price(math.Zero)
	}

	one := fixed.NewI(1, 0)
	invLeverage := math.Div(one, leverage)
	maintenance := fixed.NewF(constants.MM_RATIO)
	ratio := math.Sub(invLeverage, maintenance)
	if math.Sign(size) > 0 {
		// Long: entry * (1 - (1/leverage - MM))
		return types.Price(math.Mul(entryPrice, math.Sub(one, ratio)))
	}
	// Short: entry * (1 + (1/leverage - MM))
	return types.Price(math.Mul(entryPrice, math.Add(one, ratio)))
}

// ShouldLiquidate checks if current price crosses liquidation price.
func ShouldLiquidate(currentPrice types.Price, liqPrice types.Price, size types.Quantity) bool {
	if math.Sign(liqPrice) == 0 {
		return false
	}
	if math.Sign(size) > 0 {
		return math.Cmp(currentPrice, liqPrice) <= 0
	}
	return math.Cmp(currentPrice, liqPrice) >= 0
}
