package price

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
