package history

const (
	KIND_ORDER_CLOSED byte = 1
	KIND_TRADE        byte = 2
	KIND_PNL          byte = 3
)

type OrderClosed struct {
	OrderID    int64
	UserID     uint64
	Symbol     string
	Category   int8
	Side       int8
	Type       int8
	Status     int8
	Price      int64
	Quantity   int64
	Filled     int64
	ClosedAt   uint64
	OrderLink  int64
	ReduceOnly bool
}

type Trade struct {
	TradeID    int64
	Symbol     string
	Category   int8
	TakerID    uint64
	MakerID    uint64
	TakerOrder int64
	MakerOrder int64
	Price      int64
	Quantity   int64
	ExecutedAt uint64
}

type PnL struct {
	UserID    uint64
	Symbol    string
	Category  int8
	Realized  int64
	FeePaid   int64
	CreatedAt uint64
}
