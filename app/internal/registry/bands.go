package registry

import (
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/robaho/fixed"
)

type PriceBand struct {
	MinPrice          types.Price
	PricePrecision    int8
	QuantityPrecision int8
	TickSize          types.Price
	StepSize          types.Quantity
	MinQty            types.Quantity
	MinNotional       types.Quantity
}

var PriceBands = []PriceBand{
	{
		MinPrice:          types.Price(fixed.MustParse("1000")),
		PricePrecision:    2,
		QuantityPrecision: 6,
		TickSize:          types.Price(fixed.MustParse("0.01")),
		StepSize:          types.Quantity(fixed.MustParse("0.000001")),
		MinQty:            types.Quantity(fixed.MustParse("0.000001")),
		MinNotional:       types.Quantity(fixed.MustParse("5")),
	},
	{
		MinPrice:          types.Price(fixed.MustParse("100")),
		PricePrecision:    3,
		QuantityPrecision: 6,
		TickSize:          types.Price(fixed.MustParse("0.001")),
		StepSize:          types.Quantity(fixed.MustParse("0.000001")),
		MinQty:            types.Quantity(fixed.MustParse("0.000001")),
		MinNotional:       types.Quantity(fixed.MustParse("5")),
	},
	{
		MinPrice:          types.Price(fixed.MustParse("1")),
		PricePrecision:    4,
		QuantityPrecision: 6,
		TickSize:          types.Price(fixed.MustParse("0.0001")),
		StepSize:          types.Quantity(fixed.MustParse("0.000001")),
		MinQty:            types.Quantity(fixed.MustParse("0.000001")),
		MinNotional:       types.Quantity(fixed.MustParse("5")),
	},
	{
		MinPrice:          types.Price(fixed.MustParse("0.01")),
		PricePrecision:    5,
		QuantityPrecision: 0,
		TickSize:          types.Price(fixed.MustParse("0.00001")),
		StepSize:          types.Quantity(fixed.MustParse("1")),
		MinQty:            types.Quantity(fixed.MustParse("1")),
		MinNotional:       types.Quantity(fixed.MustParse("5")),
	},
	{
		MinPrice:          types.Price(fixed.MustParse("0")),
		PricePrecision:    8,
		QuantityPrecision: 0,
		TickSize:          types.Price(fixed.MustParse("0.00000001")),
		StepSize:          types.Quantity(fixed.MustParse("1")),
		MinQty:            types.Quantity(fixed.MustParse("1")),
		MinNotional:       types.Quantity(fixed.MustParse("0.1")),
	},
}

func GetPriceBand(price types.Price) *PriceBand {
	// Return a pointer to the slice element to avoid range-copy pointer bugs.
	for i := range PriceBands {
		if math.Cmp(price, PriceBands[i].MinPrice) >= 0 {
			return &PriceBands[i]
		}
	}
	return &PriceBands[len(PriceBands)-1]
}
