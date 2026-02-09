package registry

import (
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/robaho/fixed"
)

func FromSymbol(symbol string, lastPrice float64, assetType string) *types.Instrument {
	base := GetBaseAsset(symbol)
	quote := GetQuoteAsset(symbol)
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
