package snapshot

import (
	"bytes"
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func TestEncodeDecode(t *testing.T) {
	original := &Snapshot{
		TakenAt:     123,
		IDGenLastMS: 1704067200123,
		IDGenSeq:    7,
		Instruments: []Instrument{{Symbol: "BTCUSDT", BaseAsset: "BTC", QuoteAsset: "USDT", Category: 0}},
		Prices:      []Price{{Symbol: "BTCUSDT", Price: 50000}},
		Users: []User{{
			UserID:    1,
			Balances:  []Balance{{Asset: "USDT", Buckets: [3]int64{100, 0, 0}}},
			Positions: []Position{{Symbol: "BTCUSDT", Size: 10, Side: 0, EntryPrice: 50000, Leverage: 5}},
		}},
		Orders: []types.Order{{ID: 1, UserID: 1, Symbol: "BTCUSDT", Category: 0, Side: 0, Type: 0, TIF: 0, Status: 0, Price: 50000, Quantity: 10}},
	}
	var buf bytes.Buffer
	if err := Encode(&buf, original); err != nil {
		t.Fatalf("encode failed: %v", err)
	}
	decoded, err := Decode(&buf)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if decoded.IDGenLastMS != original.IDGenLastMS {
		t.Fatalf("expected idGenLastMS %d", original.IDGenLastMS)
	}
	if decoded.IDGenSeq != original.IDGenSeq {
		t.Fatalf("expected idGenSeq %d", original.IDGenSeq)
	}
	if len(decoded.Orders) != 1 {
		t.Fatalf("expected 1 order")
	}
}
