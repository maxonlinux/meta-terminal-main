package risk

import (
	"math/big"

	"github.com/maxonlinux/meta-terminal-go/internal/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

// Risk contains all risk metrics for a position.
type Risk struct {
	IM       *big.Int   // Initial Margin
	MM       *big.Int   // Maintenance Margin
	LiqPrice *big.Int   // Liquidation Price
	MMRatio  *big.Float // Margin Ratio
}

// LiqPrice calculates liquidation price for isolated mode.
// Bybit-style formula with 10% Maintenance Margin ratio.
// Long: liqPrice = entryPrice * (1 + MM_ratio - 1/leverage)
// Short: liqPrice = entryPrice * (1 - MM_ratio + 1/leverage)
func LiqPrice(entryPrice types.Price, size types.Quantity, leverage int8) *big.Int {
	l := int64(leverage)
	if l <= 0 {
		l = constants.DEFAULT_LEVERAGE
	}

	entry := big.NewInt(int64(entryPrice))
	lev := big.NewInt(l)

	if size > 0 {
		factor := new(big.Int).Mul(big.NewInt(11), lev)
		factor.Sub(factor, big.NewInt(10))
		result := new(big.Int).Mul(factor, entry)
		return new(big.Int).Quo(result, new(big.Int).Mul(big.NewInt(10), lev))
	} else {
		factor := new(big.Int).Mul(big.NewInt(9), lev)
		factor.Add(factor, big.NewInt(10))
		result := new(big.Int).Mul(factor, entry)
		return new(big.Int).Quo(result, new(big.Int).Mul(big.NewInt(10), lev))
	}
}

func newFloat(x float64) *big.Float {
	return new(big.Float).SetFloat64(x)
}

// IM calculates Initial Margin.
func IM(size, price, leverage int64) *big.Int {
	absSize := size
	if absSize < 0 {
		absSize = -size
	}
	notional := new(big.Int).Mul(big.NewInt(absSize), big.NewInt(price))
	l := leverage
	if l <= 0 {
		l = constants.DEFAULT_LEVERAGE
	}
	return new(big.Int).Quo(notional, big.NewInt(l))
}

// MM calculates Maintenance Margin.
func MM(size, price int64) *big.Int {
	absSize := size
	if absSize < 0 {
		absSize = -absSize
	}
	notional := new(big.Int).Mul(big.NewInt(absSize), big.NewInt(price))
	return new(big.Int).Quo(notional, big.NewInt(10))
}

// CheckLiquidation checks if a position should be liquidated.
func CheckLiquidation(p *types.Position, lastPrice types.Price) bool {
	if p.Size == 0 {
		return false
	}

	liqPrice := LiqPrice(p.EntryPrice, p.Size, p.Leverage)
	lp := liqPrice.Int64()

	if p.Size > 0 {
		return int64(lastPrice) <= lp
	}
	return int64(lastPrice) >= lp
}

// CalculateRisk calculates all risk metrics for a position.
func CalculateRisk(p *types.Position, lastPrice types.Price) Risk {
	if p.Size == 0 {
		return Risk{
			IM:       big.NewInt(0),
			MM:       big.NewInt(0),
			LiqPrice: big.NewInt(0),
			MMRatio:  newFloat(1.0),
		}
	}

	im := IM(int64(p.Size), int64(p.EntryPrice), int64(p.Leverage))
	mm := MM(int64(p.Size), int64(p.EntryPrice))
	liqPrice := LiqPrice(p.EntryPrice, p.Size, p.Leverage)
	mmRatio := MMRatio(im, mm, int64(p.Size), int64(p.EntryPrice), int64(lastPrice))

	return Risk{
		IM:       im,
		MM:       mm,
		LiqPrice: liqPrice,
		MMRatio:  mmRatio,
	}
}

// RequiredMargin calculates margin required for an order.
func RequiredMargin(qty, price types.Quantity, leverage, category int8) *big.Int {
	if category == constants.CATEGORY_SPOT {
		return big.NewInt(0)
	}

	l := int64(leverage)
	if l <= 0 {
		l = constants.DEFAULT_LEVERAGE
	}

	notional := new(big.Int).Mul(big.NewInt(int64(qty)), big.NewInt(int64(price)))
	return new(big.Int).Quo(notional, big.NewInt(l))
}

// PnL calculates unrealized profit and loss.
func PnL(size types.Quantity, entryPrice, lastPrice types.Price) *big.Int {
	priceDiff := new(big.Int).Sub(big.NewInt(int64(lastPrice)), big.NewInt(int64(entryPrice)))
	return new(big.Int).Mul(big.NewInt(int64(size)), priceDiff)
}

// MMRatio calculates Margin Ratio = MM / (IM + PnL).
func MMRatio(im, mm *big.Int, size, entryPrice, lastPrice int64) *big.Float {
	mmFloat := new(big.Float).SetInt(mm)

	var pnl *big.Int
	if size > 0 {
		pnl = new(big.Int).Mul(big.NewInt(size), new(big.Int).Sub(big.NewInt(lastPrice), big.NewInt(entryPrice)))
	} else {
		pnl = new(big.Int).Mul(big.NewInt(-size), new(big.Int).Sub(big.NewInt(entryPrice), big.NewInt(lastPrice)))
	}

	marginBalance := new(big.Int).Add(im, pnl)
	if marginBalance.Sign() <= 0 {
		return new(big.Float).SetFloat64(1.0)
	}

	marginFloat := new(big.Float).SetInt(marginBalance)
	return new(big.Float).Quo(mmFloat, marginFloat)
}
