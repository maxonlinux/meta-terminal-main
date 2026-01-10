package clearing

import (
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func BenchmarkClearing_ProcessSpotTrade(b *testing.B) {
	svc, _ := New(Config{
		NATSURL:      "nats://localhost:4222",
		StreamPrefix: "bench",
	})
	defer svc.Close()

	trade := &types.Trade{
		ID:           types.TradeID(1),
		Symbol:       "BTCUSDT",
		Category:     constants.CATEGORY_SPOT,
		Price:        types.Price(50000),
		Quantity:     types.Quantity(100),
		TakerID:      1,
		MakerID:      2,
		TakerOrderID: types.OrderID(1),
		MakerOrderID: types.OrderID(2),
		ExecutedAt:   types.NowNano(),
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		svc.processSpotTrade(trade)
	}
}

func BenchmarkClearing_ProcessLinearTrade(b *testing.B) {
	svc, _ := New(Config{
		NATSURL:      "nats://localhost:4222",
		StreamPrefix: "bench",
	})
	defer svc.Close()

	trade := &types.Trade{
		ID:           types.TradeID(1),
		Symbol:       "BTCUSDT",
		Category:     constants.CATEGORY_LINEAR,
		Price:        types.Price(50000),
		Quantity:     types.Quantity(100),
		TakerID:      1,
		MakerID:      2,
		TakerOrderID: types.OrderID(1),
		MakerOrderID: types.OrderID(2),
		ExecutedAt:   types.NowNano(),
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		svc.processLinearTrade(trade)
	}
}

func BenchmarkClearing_GetQuoteAsset(b *testing.B) {
	svc, _ := New(Config{
		NATSURL:      "nats://localhost:4222",
		StreamPrefix: "bench",
	})
	defer svc.Close()

	symbols := []string{"BTCUSDT", "ETHUSDT", "SOLUSDT", "BTCUSD", "ETHUSD"}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		for _, s := range symbols {
			svc.getQuoteAsset(s)
		}
	}
}

func BenchmarkClearing_GetBaseAsset(b *testing.B) {
	svc, _ := New(Config{
		NATSURL:      "nats://localhost:4222",
		StreamPrefix: "bench",
	})
	defer svc.Close()

	symbols := []string{"BTCUSDT", "ETHUSDT", "SOLUSDT", "BTCUSD", "ETHUSD"}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		for _, s := range symbols {
			svc.getBaseAsset(s)
		}
	}
}
