package snapshot

import "github.com/anomalyco/meta-terminal-go/internal/types"

type Snapshot struct {
	TakenAt     uint64
	IDGenLastMS uint64
	IDGenSeq    uint16
	Instruments []Instrument
	Prices      []Price
	Users       []User
	Orders      []types.Order
}

type Instrument struct {
	Symbol     string
	BaseAsset  string
	QuoteAsset string
	Category   int8
}

type Price struct {
	Symbol string
	Price  types.Price
}

type User struct {
	UserID    types.UserID
	Balances  []Balance
	Positions []Position
}

type Balance struct {
	Asset   string
	Buckets [3]int64
}

type Position struct {
	Symbol            string
	Size              types.Quantity
	Side              int8
	EntryPrice        types.Price
	Leverage          int8
	InitialMargin     int64
	MaintenanceMargin int64
	LiquidationPrice  types.Price
	Version           int64
}
