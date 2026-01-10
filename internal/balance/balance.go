package balance

import (
	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func GetQuoteAsset(symbol string) string {
	quotes := []string{"USDT", "USD", "USDC", "BUSD"}
	for _, q := range quotes {
		if len(symbol) > len(q) && symbol[len(symbol)-len(q):] == q {
			return q
		}
	}
	return "USD"
}

func GetBaseAsset(symbol string) string {
	quotes := []string{"USDT", "USD", "USDC", "BUSD"}
	for _, q := range quotes {
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
		// Long: liquidation when price drops
		// Liq = Entry × (1 - 1/L + MM_RATIO)
		ratio := 1.0/float64(leverage) - constants.MM_RATIO
		return int64(float64(entryPrice) * (1 - ratio))
	} else {
		// Short: liquidation when price rises
		// Liq = Entry × (1 + 1/L - MM_RATIO)
		ratio := 1.0/float64(leverage) - constants.MM_RATIO
		return int64(float64(entryPrice) * (1 + ratio))
	}
}

func CalculateRPNL(size int64, entryPrice int64, exitPrice int64, side int8) int64 {
	if side == constants.SIDE_LONG {
		return (exitPrice - entryPrice) * size
	}
	return (entryPrice - exitPrice) * size
}
