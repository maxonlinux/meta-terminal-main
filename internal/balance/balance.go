package balance

import (
	"strings"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

var (
	defaultQuoteAssets = []string{"USDT", "USD", "USDC", "BUSD"}
	quoteAssets        []string
)

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

func QuoteAssets() []string {
	if len(quoteAssets) > 0 {
		return quoteAssets
	}
	return defaultQuoteAssets
}

func GetQuoteAsset(symbol string) string {
	for _, q := range QuoteAssets() {
		if len(symbol) > len(q) && symbol[len(symbol)-len(q):] == q {
			return q
		}
	}
	return "USD"
}

func GetBaseAsset(symbol string) string {
	for _, q := range QuoteAssets() {
		if len(symbol) > len(q) && symbol[len(symbol)-len(q):] == q {
			return symbol[:len(symbol)-len(q)]
		}
	}
	return symbol
}

func CalculateReserveAmount(symbol string, category int8, side int8, qty types.Quantity, price types.Price, leverage int8) (int64, string) {
	if category == constants.CATEGORY_SPOT {
		if side == constants.ORDER_SIDE_BUY {
			return int64(qty) * int64(price), GetQuoteAsset(symbol)
		}
		return int64(qty), GetBaseAsset(symbol)
	}
	effectiveLeverage := leverage
	if effectiveLeverage <= 0 {
		effectiveLeverage = constants.DEFAULT_LEVERAGE
	}
	return int64(qty) * int64(price) / int64(effectiveLeverage), GetQuoteAsset(symbol)
}

func CalculateInitialMargin(qty types.Quantity, price types.Price, leverage int8) int64 {
	if leverage <= 0 {
		leverage = constants.DEFAULT_LEVERAGE
	}
	return int64(qty) * int64(price) / int64(leverage)
}

func CalculateMaintenanceMargin(initialMargin int64) int64 {
	return int64(float64(initialMargin) * constants.MM_RATIO)
}

func CalculateLiquidationPrice(entryPrice int64, leverage int8, side int8) int64 {
	if leverage <= 0 {
		leverage = constants.DEFAULT_LEVERAGE
	}

	if side == constants.SIDE_LONG {
		ratio := 1.0/float64(leverage) - constants.MM_RATIO
		return int64(float64(entryPrice) * (1 - ratio))
	} else {
		ratio := 1.0/float64(leverage) - constants.MM_RATIO
		return int64(float64(entryPrice) * (1 + ratio))
	}
}
