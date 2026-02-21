package types

type PlaceOrderRequest struct {
	UserID         UserID
	Symbol         string
	Category       int8
	Origin         int8
	Side           int8
	Type           int8
	TIF            int8
	Price          Price
	Quantity       Quantity
	TriggerPrice   Price
	ReduceOnly     bool
	CloseOnTrigger bool
	StopOrderType  int8
}

// Order represents a trading order - canonical structure used throughout the system
// Single struct used by Store, OrderBook, and TriggerMonitor ensures consistency
type Order struct {
	ID       OrderID
	UserID   UserID
	Symbol   string
	Category int8 // 0=SPOT, 1=LINEAR
	Origin   int8 // 0=USER, 1=SYSTEM

	Side int8 // 0=BUY, 1=SELL
	Type int8 // 0=LIMIT, 1=MARKET
	TIF  int8 // Time In Force

	Status int8 // Order status (see constants)

	Price        Price    // Order price
	Quantity     Quantity // Total quantity
	Filled       Quantity // Filled quantity
	TriggerPrice Price    // 0 = regular order, >0 = conditional order

	ReduceOnly       bool // Reduce-only flag (LINEAR only)
	CloseOnTrigger   bool // Close on trigger (CoT orders)
	StopOrderType    int8 // Stop order type for conditional orders
	TriggerDirection int8 // 1=UP, -1=DOWN, 0=NONE

	IsConditional bool // true if TriggerPrice > 0

	// Timestamps - no atomic needed in single-threaded architecture
	CreatedAt uint64
	UpdatedAt uint64
}
