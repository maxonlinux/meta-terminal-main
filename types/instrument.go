package types

type Symbol struct {
	Name         string
	Category     int8
	BaseAsset    string
	QuoteAsset   string
	PriceFilters *PriceFilters
}

type PriceFilters struct {
	PricePrecision    int8
	QuantityPrecision int8
	TickSize          int64
	StepSize          int64
	MinQty            int64
	MinNotional       int64
}
