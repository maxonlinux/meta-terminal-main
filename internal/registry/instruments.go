package registry

import (
	"strings"

	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/robaho/fixed"
)

func FromSymbol(symbol string, lastPrice float64, assetType string) *types.Instrument {
	base, quote := splitSymbol(symbol)
	band := GetPriceBand(lastPrice)

	return &types.Instrument{
		Symbol:     symbol,
		BaseAsset:  base,
		QuoteAsset: quote,
		AssetType:  assetType,
		PricePrec:  band.PricePrecision,
		QtyPrec:    band.QuantityPrecision,
		MinQty:     types.Quantity(fixed.NewF(band.MinQty)),
		MaxQty:     types.Quantity(fixed.NewF(999999999)),
		MinPrice:   types.Price(fixed.NewF(0)),
		MaxPrice:   types.Price(fixed.NewF(999999999)),
		TickSize:   types.Price(fixed.NewF(band.TickSize)),
		LotSize:    types.Quantity(fixed.NewF(band.StepSize)),
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
