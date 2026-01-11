package messaging

const (
	SUBJECT_ORDER_PLACE  = "order.place"
	SUBJECT_ORDER_EVENT  = "order.event"
	SUBJECT_ORDER_CANCEL = "order.cancel"

	SUBJECT_CLEARING_TRADE   = "clearing.trade"
	SUBJECT_CLEARING_RESERVE = "clearing.reserve"
	SUBJECT_CLEARING_RELEASE = "clearing.release"

	SUBJECT_PORTFOLIO_RESERVE = "portfolio.reserve"
	SUBJECT_PORTFOLIO_RELEASE = "portfolio.release"
	SUBJECT_PORTFOLIO_MARGIN  = "portfolio.margin"
	SUBJECT_POSITIONS_EVENT   = "positions.event"

	SUBJECT_PRICE_TICK          = "price.tick"
	SUBJECT_MARKETDATA_SNAPSHOT = "marketdata.snapshot"

	SUBJECT_USER_REGISTERED = "user.registered"
	SUBJECT_PORTFOLIO_QUERY = "portfolio.query"
	SUBJECT_OMS_QUERY       = "oms.query"

	SUBJECT_OUTBOX_ORDER = "outbox.order"
	SUBJECT_OUTBOX_TRADE = "outbox.trade"
	SUBJECT_OUTBOX_RPNL  = "outbox.rpnl"

	SUBJECT_POSITION_REDUCED = "position.reduced"
)

func OrderPlaceTopic(symbol string) string         { return "order.place." + symbol }
func OrderEventTopic(symbol string) string         { return "order.event." + symbol }
func OrderCancelTopic(symbol string) string        { return "order.cancel." + symbol }
func PriceTickTopic(symbol string) string          { return "price.tick." + symbol }
func PositionsEventTopic(symbol string) string     { return "positions.event." + symbol }
func MarketDataSnapshotTopic(symbol string) string { return "marketdata.snapshot." + symbol }
func PositionReducedTopic(symbol string) string    { return "position.reduced." + symbol }
