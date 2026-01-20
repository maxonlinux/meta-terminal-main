package clearing

import (
	"github.com/maxonlinux/meta-terminal-go/internal/registry"
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/robaho/fixed"
)

var one = fixed.NewI(1, 0)

type Portfolio interface {
	Reserve(userID types.UserID, asset string, amount types.Quantity) error
	Release(userID types.UserID, asset string, amount types.Quantity)
	ExecuteTrade(match *types.Match)
	GetPosition(userID types.UserID, symbol string) *types.Position
}

type Service struct {
	portfolio Portfolio
	registry  *registry.Registry
}

func New(portfolio Portfolio, reg *registry.Registry) *Service {
	return &Service{portfolio: portfolio, registry: reg}
}

func (s *Service) Reserve(userID types.UserID, symbol string, category int8, side int8, qty types.Quantity, price types.Price) error {
	amount, asset := CalculateReserveAmount(symbol, category, side, qty, price, s.leverageFor(userID, symbol), s.registry)
	return s.portfolio.Reserve(userID, asset, amount)
}

func (s *Service) Release(userID types.UserID, symbol string, category int8, side int8, qty types.Quantity, price types.Price) {
	amount, asset := CalculateReserveAmount(symbol, category, side, qty, price, s.leverageFor(userID, symbol), s.registry)
	s.portfolio.Release(userID, asset, amount)
}

func (s *Service) ExecuteTrade(match *types.Match) {
	if match.Category == constants.CATEGORY_SPOT {
		inst := registry.GetInstrument(s.registry, match.Symbol)
		base, quote := inst.BaseAsset, inst.QuoteAsset
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

func (s *Service) leverageFor(userID types.UserID, symbol string) types.Leverage {
	pos := s.portfolio.GetPosition(userID, symbol)
	if math.Sign(pos.Leverage) > 0 {
		return pos.Leverage
	}
	return types.Leverage(fixed.NewI(int64(constants.DEFAULT_LEVERAGE), 0))
}

func CalculateReserveAmount(symbol string, category int8, side int8, qty types.Quantity, price types.Price, leverage types.Leverage, reg *registry.Registry) (types.Quantity, string) {
	if category == constants.CATEGORY_SPOT {
		inst := registry.GetInstrument(reg, symbol)
		if side == constants.ORDER_SIDE_BUY {
			return types.Quantity(math.Mul(qty, price)), inst.QuoteAsset
		}
		return qty, inst.BaseAsset
	}

	effective := leverage
	if math.Sign(effective) <= 0 {
		effective = types.Leverage(fixed.NewI(int64(constants.DEFAULT_LEVERAGE), 0))
	}
	reserve := math.MulDiv(qty, price, effective)
	return types.Quantity(reserve), registry.GetInstrument(reg, symbol).QuoteAsset
}

func LiquidationPrice(entryPrice types.Price, leverage types.Leverage, size types.Quantity) types.Price {
	if math.Sign(entryPrice) <= 0 || math.Sign(leverage) <= 0 {
		return types.Price(math.Zero)
	}

	invLeverage := math.Div(one, leverage)
	maintenance := fixed.NewF(constants.MM_RATIO)
	ratio := math.Sub(invLeverage, maintenance)
	if math.Sign(size) > 0 {
		return types.Price(math.Mul(entryPrice, math.Sub(one, ratio)))
	}
	return types.Price(math.Mul(entryPrice, math.Add(one, ratio)))
}

func ShouldLiquidate(currentPrice types.Price, liqPrice types.Price, size types.Quantity) bool {
	if math.Sign(liqPrice) == 0 {
		return false
	}
	if math.Sign(size) > 0 {
		return math.Cmp(currentPrice, liqPrice) <= 0
	}
	return math.Cmp(currentPrice, liqPrice) >= 0
}
