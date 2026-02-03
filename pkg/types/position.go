package types

type PositionMode int8

// Leverage is stored as a fixed-point ratio (e.g. 2, 5, 10).
type Leverage = Quantity

// Position represents user's position in LINEAR market (not SPOT)
// Direction is determined by Size sign: >0 = LONG, <0 = SHORT, 0 = NONE
type Position struct {
	UserID     UserID
	Symbol     string       // BTCUSDT, ETHUSDT, etc.
	Size       Quantity     // Absolute size (positive = LONG, negative = SHORT)
	EntryPrice Price        // Average entry price
	ExitPrice  Price        // Exit price
	Mode       PositionMode // POSITION_MODE_ONE_WAY = 0 POSITION_MODE_HEDGE = 1
	MM         Quantity     // Maintenance Margin
	IM         Quantity     // Initial Margin
	LiqPrice   Price        // Liquidation price
	Leverage   Leverage     // Leverage (2, 5, 10, ... 100)
	TakeProfit Price
	StopLoss   Price
	TPOrderID  OrderID
	SLOrderID  OrderID
}
