package types

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
	Timestamp    uint64
}
