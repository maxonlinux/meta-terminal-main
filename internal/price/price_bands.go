package price

import "math"

type PriceBand struct {
	MinPrice          float64
	PricePrecision    int8
	QuantityPrecision int8
	TickSize          int64
	StepSize          int64
	MinQty            int64
	MinNotional       int64
}

var Bands = []PriceBand{
	{1000.0, 2, 6, 1, 1, 1, 500000000},
	{100.0, 3, 6, 1, 1, 1, 500000000},
	{1.0, 4, 6, 1, 1, 1, 500000000},
	{0.01, 5, 0, 1, 1, 1, 500000000},
	{0.0, 8, 0, 1, 1, 1, 10000000},
}

func GetFilters(price float64) *PriceBand {
	for _, band := range Bands {
		if price >= band.MinPrice {
			return &band
		}
	}
	return &Bands[len(Bands)-1]
}

func GetFiltersWithBounds(price float64) (*PriceBand, int64, int64) {
	for i, band := range Bands {
		if price >= band.MinPrice {
			minPrice := int64(band.MinPrice)
			if minPrice < band.TickSize {
				minPrice = band.TickSize
			}
			maxPrice := maxPriceForBand(i)
			return &band, minPrice, maxPrice
		}
	}
	last := Bands[len(Bands)-1]
	return &last, int64(last.MinPrice), math.MaxInt64
}

func maxPriceForBand(index int) int64 {
	if index == 0 {
		return math.MaxInt64
	}
	prev := Bands[index-1]
	max := int64(prev.MinPrice) - prev.TickSize
	if max <= 0 {
		return math.MaxInt64
	}
	return max
}
