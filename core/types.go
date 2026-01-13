package core

import "time"

// Basic type definitions - simple aliases, no thread safety needed in zero-lock architecture
type OrderID int64
type TradeID int64
type UserID uint64
type Price int64
type Quantity int64

// NowNano provides timestamp for the single-threaded event loop
func NowNano() uint64 { return uint64(time.Now().UnixNano()) }

// Order represents a trading order - canonical structure used throughout the system
// WHY: Single struct used by Store, OrderBook, and TriggerMonitor ensures consistency
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

	// OCO/TP/SL grouping: >0 = snowflake ID of group, -1 = no group
	OrderLinkId int64

	IsConditional bool // true if TriggerPrice > 0

	// Timestamps - no atomic needed in single-threaded architecture
	CreatedAt uint64
	UpdatedAt uint64
	ClosedAt  uint64
}

// Position represents user's position in LINEAR market
// WHY: Only exists in LINEAR market, not SPOT
type Position struct {
	UserID     UserID
	Symbol     string
	Size       Quantity // Absolute size (positive = LONG, negative = SHORT)
	Side       int8     // -1=NONE, 0=LONG, 1=SHORT
	EntryPrice Price    // Average entry price
	Leverage   int      // Leverage (1, 2, 5, 10, etc.)
}

// Balance represents user's balance for a symbol
// WHY: Three buckets (Available, Locked, Margin) track balance through order lifecycle
type Balance struct {
	UserID    UserID
	Symbol    string
	Available Quantity // Available for trading/deposit
	Locked    Quantity // Locked in orders/reserved
	Margin    Quantity // Used as margin (LINEAR only)
}

// OCOInput defines One-Cancels-the-Other order parameters
type OCOInput struct {
	Quantity   Quantity // 0 = use full position size
	TakeProfit OCOChildOrder
	StopLoss   OCOChildOrder
}

// OCOChildOrder defines individual TP/SL order parameters
type OCOChildOrder struct {
	TriggerPrice Price // Price that triggers this order
	Price        Price // Limit price for the triggered order (0 = Market)
	ReduceOnly   bool  // Always true for OCO orders
}

// OrderInput is the input structure for placing new orders
type OrderInput struct {
	UserID   UserID
	Symbol   string
	Category int8 // 0=SPOT, 1=LINEAR

	Side int8
	Type int8
	TIF  int8

	Quantity Quantity
	Price    Price

	TriggerPrice   Price
	ReduceOnly     bool
	CloseOnTrigger bool
	StopOrderType  int8

	IsConditional bool // auto-set if TriggerPrice > 0

	OCO *OCOInput `json:"oco,omitempty"`
}

// OrderResult is returned by PlaceOrder
type OrderResult struct {
	Orders    []*Order // Created orders (1 for single, 2 for OCO)
	Trades    []*Trade // Any immediate trades
	Filled    Quantity // Total filled quantity
	Remaining Quantity // Remaining quantity
	Status    int8     // Final status
}

// Trade represents an executed trade
type Trade struct {
	ID         TradeID
	Symbol     string
	Price      Price
	Quantity   Quantity
	TakerOrder *Order // Aggressor order
	MakerOrder *Order // Passive order
	Timestamp  uint64
}

// OrderRequest represents an incoming order request from API
type OrderRequest struct {
	UserID         uint64
	Symbol         string
	Category       int8 // 0=SPOT, 1=LINEAR
	Side           int8
	Type           int8
	TIF            int8
	Quantity       int64
	Price          int64
	ReduceOnly     bool
	TriggerPrice   int64
	CloseOnTrigger bool
	OCO            *OCOInput `json:"oco,omitempty"`
}

// CancelRequest represents an order cancellation request
type CancelRequest struct {
	OrderID  int64
	UserID   uint64
	Symbol   string
	Category int8
}

// PriceTick represents a price update event
type PriceTick struct {
	Symbol string
	Price  int64
}

// TradeEvent represents a trade that occurred
type TradeEvent struct {
	Symbol       string
	Price        Price
	Quantity     Quantity
	TakerOrderID OrderID
	MakerOrderID OrderID
	TakerUserID  UserID
	MakerUserID  UserID
}

// PositionUpdate represents a position change event
type PositionUpdate struct {
	UserID     uint64
	Symbol     string
	Size       int64
	Side       int8
	EntryPrice int64
	Leverage   int
}
