package snapshot

import (
	"bytes"
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func BenchmarkEncode(b *testing.B) {
	snap := &Snapshot{
		TakenAt:     1,
		IDGenLastMS: 1704067200001,
		IDGenSeq:    2,
		Instruments: []Instrument{{Symbol: "BTCUSDT", BaseAsset: "BTC", QuoteAsset: "USDT", Category: 0}},
		Prices:      []Price{{Symbol: "BTCUSDT", Price: 50000}},
		Users: []User{{
			UserID:    1,
			Balances:  []Balance{{Asset: "USDT", Buckets: [3]int64{100, 0, 0}}},
			Positions: []Position{{Symbol: "BTCUSDT", Size: 10, Side: 0, EntryPrice: 50000, Leverage: 5}},
		}},
		Orders: []types.Order{{ID: 1, UserID: 1, Symbol: "BTCUSDT", Category: 0, Side: 0, Type: 0, TIF: 0, Status: 0, Price: 50000, Quantity: 10}},
	}
	b.ReportAllocs()

	for b.Loop() {
		var buf bytes.Buffer
		if err := Encode(&buf, snap); err != nil {
			b.Fatalf("encode failed: %v", err)
		}
	}
}
