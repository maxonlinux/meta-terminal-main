package registry

import (
	"strings"

	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/robaho/fixed"
)

func FromSymbol(symbol string, lastPrice types.Price, assetType string) *types.Instrument {
	// Select price band using fixed-point price to avoid float rounding.
	base, quote := splitSymbol(symbol)
	band := GetPriceBand(lastPrice)

	return &types.Instrument{
		Symbol:     symbol,
		BaseAsset:  base,
		QuoteAsset: quote,
		AssetType:  assetType,
		PricePrec:  band.PricePrecision,
		QtyPrec:    band.QuantityPrecision,
		MinQty:     band.MinQty,
		MaxQty:     types.Quantity(fixed.NewI(999999999, 0)),
		MinPrice:   types.Price(fixed.NewI(0, 0)),
		MaxPrice:   types.Price(fixed.NewI(999999999, 0)),
		TickSize:   band.TickSize,
		LotSize:    band.StepSize,
	}
}

func splitSymbol(symbol string) (string, string) {
	quotes := []string{"USDT", "USDC", "USD"}
	fallback := "USD"
	matched := ""

	for _, quote := range quotes {
		if strings.HasSuffix(symbol, quote) {
			matched = quote
			break
		}
	}

	if matched == "" {
		return symbol, fallback
	}

	return symbol[:len(symbol)-len(matched)], matched
}
