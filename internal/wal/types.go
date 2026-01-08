package wal

const (
	EventPlaceOrder    uint8 = 1
	EventCancelOrder   uint8 = 2
	EventPriceTick     uint8 = 3
	EventSetLeverage   uint8 = 4
	EventSetBalance    uint8 = 5
	EventAddInstrument uint8 = 6
)

type Event struct {
	Type      uint8
	Timestamp uint64
	Payload   []byte
}
