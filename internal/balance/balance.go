package balance

import (
	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

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
