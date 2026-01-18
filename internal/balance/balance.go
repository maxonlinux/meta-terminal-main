package balance

import (
	"strings"

	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/robaho/fixed"
)

var (
	defaultQuoteAssets = []string{"USDT", "USD", "USDC", "BUSD"}
	quoteAssets        []string
)

// SetQuoteAssets configures custom quote assets used for symbol parsing.
func SetQuoteAssets(assets []string) {
	if len(assets) == 0 {
		quoteAssets = nil
		return
	}

	normalized := make([]string, 0, len(assets))
	for _, asset := range assets {
		if asset == "" {
			continue
		}
		normalized = append(normalized, strings.TrimSpace(asset))
	}
	quoteAssets = normalized
}

// QuoteAssets returns the configured quote asset list.
func QuoteAssets() []string {
	if len(quoteAssets) > 0 {
		return quoteAssets
	}
	return defaultQuoteAssets
}

// GetQuoteAsset extracts the quote asset from a symbol string.
func GetQuoteAsset(symbol string) string {
	for _, q := range QuoteAssets() {
		if len(symbol) > len(q) && symbol[len(symbol)-len(q):] == q {
			return q
		}
	}
	return "USD"
}

// GetBaseAsset extracts the base asset from a symbol string.
func GetBaseAsset(symbol string) string {
	for _, q := range QuoteAssets() {
		if len(symbol) > len(q) && symbol[len(symbol)-len(q):] == q {
			return symbol[:len(symbol)-len(q)]
		}
	}
	return symbol
}

// DefaultLeverage returns the default leverage ratio.
func DefaultLeverage() types.Leverage {
	return types.Leverage(fixed.NewI(int64(constants.DEFAULT_LEVERAGE), 0))
}

// CalculateReserveAmount computes the reservation amount for a new order.
func CalculateReserveAmount(symbol string, category int8, side int8, qty types.Quantity, price types.Price, leverage types.Leverage) (types.Quantity, string) {
	if category == constants.CATEGORY_SPOT {
		if side == constants.ORDER_SIDE_BUY {
			return types.Quantity(math.Mul(qty, price)), GetQuoteAsset(symbol)
		}
		return qty, GetBaseAsset(symbol)
	}

	effective := leverage
	if math.Sign(effective) <= 0 {
		effective = DefaultLeverage()
	}
	reserve := math.MulDiv(qty, price, effective)
	return types.Quantity(reserve), GetQuoteAsset(symbol)
}
