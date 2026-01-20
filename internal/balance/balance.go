package balance

import (
	"strings"
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
