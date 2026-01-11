package registry

import (
	"math"

	"github.com/anomalyco/meta-terminal-go/internal/balance"
	"github.com/anomalyco/meta-terminal-go/internal/price"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func FromSymbol(symbol string, lastPrice int64) *types.Instrument {
	base := balance.GetBaseAsset(symbol)
	quote := balance.GetQuoteAsset(symbol)
	band, minPrice, maxPrice := price.GetFiltersWithBounds(float64(lastPrice))

	return &types.Instrument{
		Symbol:     symbol,
		BaseAsset:  base,
		QuoteAsset: quote,
		PricePrec:  band.PricePrecision,
		QtyPrec:    band.QuantityPrecision,
		MinQty:     band.MinQty,
		MaxQty:     math.MaxInt64,
		MinPrice:   minPrice,
		MaxPrice:   maxPrice,
		TickSize:   band.TickSize,
		LotSize:    band.StepSize,
	}
}
