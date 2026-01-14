package types

import "time"

// Basic type aliases for domain concepts
type OrderID int64
type TradeID int64
type UserID uint64
type Price int64
type Quantity int64

// NowNano returns the current time in nanoseconds since epoch.
// Used for high-precision timestamps in trading events.
func NowNano() uint64 {
	return uint64(time.Now().UnixNano())
}

// Order represents a trading order - canonical structure used throughout the system
// Single struct used by Store, OrderBook, and TriggerMonitor ensures consistency
type Order struct {
	ID       OrderID
	UserID   UserID
	Symbol   string
	Category int8 // 0=SPOT, 1=LINEAR

	Side int8 // 0=BUY, 1=SELL
	Type int8 // 0=LIMIT, 1=MARKET
	TIF  int8 // Time In Force

	Status int8 // Order status (see constants)

	Price        Price    // Order price
	Quantity     Quantity // Total quantity
	Filled       Quantity // Filled quantity
	TriggerPrice Price    // 0 = regular order, >0 = conditional order

	ReduceOnly     bool // Reduce-only flag (LINEAR only)
	CloseOnTrigger bool // Close on trigger (CoT orders)
	StopOrderType  int8 // Stop order type for conditional orders

	IsConditional bool // true if TriggerPrice > 0

	// Timestamps - no atomic needed in single-threaded architecture
	CreatedAt uint64
	UpdatedAt uint64
	ClosedAt  uint64
}

// Remaining returns unfilled quantity
func (o *Order) Remaining() Quantity { return o.Quantity - o.Filled }

// Position represents user's position in LINEAR market (not SPOT)
type Position struct {
	UserID     UserID
	Symbol     string   // BTCUSDT, ETHUSDT, etc.
	Size       Quantity // Absolute size (positive = LONG, negative = SHORT)
	Side       int8     // -1=NONE, 0=LONG, 1=SHORT
	EntryPrice Price    // Average entry price
	ExitPrice  Price    // Exit price
	Mode       int      // Position Mode (0=ISOLATED, 1=CROSS)
	MM         int      // Maintenance Margin ratio (percent)
	IM         int      // Initial Margin
	LiqPrice   Price    // Liquidation price
	Leverage   int      // Leverage (2, 5, 10, ... 100)
}

// Trade represents an executed trade
type Trade struct {
	ID           TradeID
	Symbol       string
	Category     int8 // 0=SPOT, 1=LINEAR
	Price        Price
	Quantity     Quantity
	TakerOrder   *Order // Aggressor order
	MakerOrder   *Order // Passive order
	TakerOrderID OrderID
	MakerOrderID OrderID
	TakerID      UserID
	MakerID      UserID
	ExecutedAt   uint64 // Execution timestamp
	Timestamp    uint64
}

// UserBalance represents user's total balance for an asset
type UserBalance struct {
	UserID    UserID
	Asset     string
	Available Quantity
	Locked    Quantity
	Margin    Quantity
}

// PriceTick represents a price update event
type PriceTick struct {
	Symbol string
	Price  int64
	Bid    int64
	Ask    int64
	Volume int64
	Time   int64
}

// Instrument represents a trading instrument with precision settings
type Instrument struct {
	Symbol     string
	BaseAsset  string
	QuoteAsset string
	PricePrec  int8  // Price precision (decimal places)
	QtyPrec    int8  // Quantity precision (decimal places)
	MinQty     int64 // Minimum quantity
	MaxQty     int64 // Maximum quantity
	MinPrice   int64 // Minimum price
	MaxPrice   int64 // Maximum price
	TickSize   int64 // Price tick size
	LotSize    int64 // Lot size

	// Market data - updated in real-time
	LastPrice int64  // Last traded price (from NATS/registry)
	UpdatedAt uint64 // Timestamp of last update
}

// Match represents a single match between two orders
type Match struct {
	Trade Trade
	Maker *Order
}

// OrderResult represents the result of an order matching operation.
// Used by the matching engine to return both trades and remaining orders.
type OrderResult struct {
	Orders    []*Order // Orders modified during matching
	Trades    []*Trade // Trades created during matching
	OrdersBuf []*Order // Pre-allocated order buffer for pooling
	TradesBuf []Trade  // Pre-allocated trade buffer for pooling
}

// Trigger represents a conditional order trigger that monitors price levels.
// When triggered, the conditional order is converted to a regular order.
type Trigger struct {
	Order        *Order // The conditional order to activate
	Symbol       string // Symbol being monitored
	TriggerPrice Price  // Price level that triggers the order
	Side         int8   // Order side (BUY/SELL)
	IsActive     bool   // Whether trigger is currently monitoring
}
