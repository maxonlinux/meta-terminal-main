package types

type Position struct {
	UserID     UserID
	Symbol     string
	Size       Quantity
	Side       int8
	EntryPrice Price
	Leverage   int8

	InitialMargin     int64
	MaintenanceMargin int64
	LiquidationPrice  Price

	RealizedPnl int64
	Version     int64
}

const (
	SIDE_NONE  int8 = -1
	SIDE_LONG  int8 = 0
	SIDE_SHORT int8 = 1
)
