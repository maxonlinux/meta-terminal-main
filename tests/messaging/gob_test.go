package messaging_test

import (
	"reflect"
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/messaging"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func TestGobRoundTripOrderInput(t *testing.T) {
	input := types.OrderInput{
		UserID:       1,
		Symbol:       "BTCUSDT",
		Category:     1,
		Side:         0,
		Type:         0,
		TIF:          0,
		Quantity:     5,
		Price:        10000,
		TriggerPrice: 0,
		ReduceOnly:   false,
	}
	data, err := messaging.EncodeGob(input)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	var decoded types.OrderInput
	if err := messaging.DecodeGob(data, &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !reflect.DeepEqual(input, decoded) {
		t.Fatalf("roundtrip mismatch: %+v != %+v", input, decoded)
	}
}

func TestGobRoundTripTradeEvent(t *testing.T) {
	event := types.TradeEvent{
		TradeID:      123,
		Symbol:       "BTCUSDT",
		Category:     1,
		TakerID:      1,
		MakerID:      2,
		TakerOrderID: 10,
		MakerOrderID: 11,
		Price:        10000,
		Quantity:     2,
		ExecutedAt:   123456789,
	}
	data, err := messaging.EncodeGob(event)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	var decoded types.TradeEvent
	if err := messaging.DecodeGob(data, &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !reflect.DeepEqual(event, decoded) {
		t.Fatalf("roundtrip mismatch: %+v != %+v", event, decoded)
	}
}
