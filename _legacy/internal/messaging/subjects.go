package messaging

// NATS subjects (strict lowercase.dot, symbol case preserved)
// Each shard subscribes only to its own symbol:
//   last token = symbol (e.g., "order.place.BTCUSDT")
// Service-type prefix keeps topics organized per domain.

const (
	// Orders
	SubjectOrderPlace  = "order.place"  // + .<symbol> -> Gateway publishes here
	SubjectOrderEvent  = "order.event"  // + .<symbol> -> OMS publishes status/fill/cancel
	SubjectOrderCancel = "order.cancel" // + .<symbol> -> OMS publishes when cancel initiated
	SubjectOrderTrim   = "order.trim"   // + .<symbol> -> OMS publishes when trimming order

	// Clearing (single shared subject; processing can be sharded by symbol in service logic)
	SubjectClearingTrade   = "clearing.trade"   // broadcast to all clearing instances
	SubjectClearingReserve = "clearing.reserve" // request-reply from OMS (binary payload includes symbol)
	SubjectClearingRelease = "clearing.release" // broadcast to all clearing instances

	// Portfolio/Positions (single shared subject; handler keys by symbol in payload)
	SubjectPortfolioReserve = "portfolio.reserve"
	SubjectPortfolioRelease = "portfolio.release"
	SubjectPortfolioMargin  = "portfolio.margin"
	SubjectPortfolioGet     = "portfolio.get"
	SubjectPortfolioGetPos  = "portfolio.get.position"
	SubjectPositionsEvent   = "positions.event" // + .<symbol> -> updates

	// Market Data
	SubjectPriceTick          = "price.tick"          // + .<symbol> -> price updates per symbol
	SubjectMarketDataSnapshot = "marketdata.snapshot" // + .<symbol> -> periodic snapshots

	// Gateway/Session
	SubjectUserRegistered = "user.registered" // no symbol; gateway keeps sessions map
	SubjectPortfolioQuery = "portfolio.query" // request-reply (gateway)
	SubjectOMSQuery       = "oms.query"       // request-reply (gateway)
)

// Topic builders with symbol suffix for shard filtering
func OrderPlaceTopic(symbol string) string         { return "order.place." + symbol }
func OrderEventTopic(symbol string) string         { return "order.event." + symbol }
func OrderCancelTopic(symbol string) string        { return "order.cancel." + symbol }
func OrderTrimTopic(symbol string) string          { return "order.trim." + symbol }
func PriceTickTopic(symbol string) string          { return "price.tick." + symbol }
func PositionsEventTopic(symbol string) string     { return "positions.event." + symbol }
func MarketDataSnapshotTopic(symbol string) string { return "marketdata.snapshot." + symbol }
