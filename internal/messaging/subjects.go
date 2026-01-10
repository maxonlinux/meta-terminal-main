package messaging

const (
	SubjectOrderPlace  = "order.place"
	SubjectOrderEvent  = "order.event"
	SubjectOrderCancel = "order.cancel"

	SubjectClearingTrade   = "clearing.trade"
	SubjectClearingReserve = "clearing.reserve"
	SubjectClearingRelease = "clearing.release"

	SubjectPortfolioReserve = "portfolio.reserve"
	SubjectPortfolioRelease = "portfolio.release"
	SubjectPortfolioMargin  = "portfolio.margin"
	SubjectPositionsEvent   = "positions.event"

	SubjectPriceTick          = "price.tick"
	SubjectMarketDataSnapshot = "marketdata.snapshot"

	SubjectUserRegistered = "user.registered"
	SubjectPortfolioQuery = "portfolio.query"
	SubjectOMSQuery       = "oms.query"

	SubjectOutboxOrder = "outbox.order"
	SubjectOutboxTrade = "outbox.trade"
	SubjectOutboxRPNL  = "outbox.rpnl"

	SubjectPositionReduced = "position.reduced"
)

func OrderPlaceTopic(symbol string) string         { return "order.place." + symbol }
func OrderEventTopic(symbol string) string         { return "order.event." + symbol }
func OrderCancelTopic(symbol string) string        { return "order.cancel." + symbol }
func PriceTickTopic(symbol string) string          { return "price.tick." + symbol }
func PositionsEventTopic(symbol string) string     { return "positions.event." + symbol }
func MarketDataSnapshotTopic(symbol string) string { return "marketdata.snapshot." + symbol }
func PositionReducedTopic(symbol string) string    { return "position.reduced." + symbol }
