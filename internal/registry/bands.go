package registry

type PriceBand struct {
	MinPrice          float64
	PricePrecision    int8
	QuantityPrecision int8
	TickSize          float64
	StepSize          float64
	MinQty            float64
	MinNotional       float64
}

var PriceBands = []PriceBand{
	{1000.0, 2, 6, 0.01, 0.000001, 0.000001, 5},
	{100.0, 3, 6, 0.001, 0.000001, 0.000001, 5},
	{1.0, 4, 6, 0.0001, 0.000001, 0.000001, 5},
	{0.01, 5, 0, 0.00001, 1, 1, 5},
	{0.0, 8, 0, 0.00000001, 1, 1, 0.1},
}

func GetPriceBand(price float64) *PriceBand {
	for _, band := range PriceBands {
		if price >= band.MinPrice {
			return &band
		}
	}
	return &PriceBands[len(PriceBands)-1]
}

func GetQuoteAsset(symbol string) string {
	if len(symbol) >= 4 {
		quote := symbol[len(symbol)-4:]
		if quote == "USDT" || quote == "USDC" || quote == "BUSD" {
			return quote
		}
	}
	if len(symbol) >= 3 {
		quote := symbol[len(symbol)-3:]
		if quote == "USD" {
			return quote
		}
	}
	return "USD"
}

func GetBaseAsset(symbol string) string {
	quote := GetQuoteAsset(symbol)
	baseLen := len(symbol) - len(quote)
	if baseLen <= 0 {
		return symbol
	}
	return symbol[:baseLen]
}
