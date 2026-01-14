package types

import (
	"strings"
	"time"
)

// NowNano returns current time in nanoseconds
func NowNano() uint64 { return uint64(time.Now().UnixNano()) }

// configuredQuotes holds user-provided quote asset list
var configuredQuotes []string

// ConfigureQuoteAssets sets the list of recognized quote assets
func ConfigureQuoteAssets(assets []string) {
	if len(assets) == 0 {
		configuredQuotes = nil
		return
	}
	norm := make([]string, 0, len(assets))
	for _, s := range assets {
		if s = strings.TrimSpace(s); s != "" {
			norm = append(norm, s)
		}
	}
	configuredQuotes = norm
}

// quoteAssets returns the list of quote assets to check
func quoteAssets() []string {
	if len(configuredQuotes) > 0 {
		return configuredQuotes
	}
	return []string{"USDT", "USDC", "USD", "BUSD"}
}

// GetQuoteAsset extracts the quote currency from a symbol
// Examples: "BTCUSDT" -> "USDT", "ETHUSD" -> "USD"
func GetQuoteAsset(symbol string) string {
	for _, q := range quoteAssets() {
		if len(symbol) > len(q) && symbol[len(symbol)-len(q):] == q {
			return q
		}
	}
	return "USD"
}

// GetBaseAsset extracts the base currency from a symbol
// Examples: "BTCUSDT" -> "BTC", "ETHUSD" -> "ETH"
func GetBaseAsset(symbol string) string {
	return symbol[:len(symbol)-len(GetQuoteAsset(symbol))]
}
