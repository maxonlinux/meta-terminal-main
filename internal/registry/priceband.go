package registry

import "math"

// PriceBand defines trading constraints based on price range
type PriceBand struct {
	MinPrice          float64
	PricePrecision    int8
	QuantityPrecision int8
	TickSize          int64
	StepSize          int64
	MinQty            int64
	MinNotional       int64
}

// Bands define price filter tiers for different price ranges
// Price ranges are inclusive of the minimum, exclusive of the next band
var Bands = []PriceBand{
	{1000.0, 2, 6, 1, 1, 1, 500000000},
	{100.0, 3, 6, 1, 1, 1, 500000000},
	{1.0, 4, 6, 1, 1, 1, 500000000},
	{0.01, 5, 0, 1, 1, 1, 500000000},
	{0.0, 8, 0, 1, 1, 1, 10000000},
}

// GetBand returns the price band for a given price level
func GetBand(price float64) *PriceBand {
	for _, b := range Bands {
		if price >= b.MinPrice {
			return &b
		}
	}

	// fallback to the last band
	return &Bands[len(Bands)-1]
}

// GetBandBounds returns the price band and valid price range for a price level
func GetBandBounds(price float64) (*PriceBand, int64, int64) {
	for i, b := range Bands {
		if price >= b.MinPrice {
			minPrice := max(int64(b.MinPrice), b.TickSize)
			return &b, minPrice, maxPriceForBand(i)
		}
	}
	last := Bands[len(Bands)-1]
	return &last, int64(last.MinPrice), math.MaxInt64
}

// maxPriceForBand returns the maximum valid price for a band
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
