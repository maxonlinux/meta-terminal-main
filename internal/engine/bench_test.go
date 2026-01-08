package engine

import (
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func BenchmarkPlaceOrderGTC(b *testing.B) {
	eng := newTestEngine()
	_ = eng.SetBalance(1, "USDT", 1_000_000_000)
	b.ReportAllocs()

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_SPOT,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 10,
		Price:    50000,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res, err := eng.PlaceOrder(input)
		if err != nil {
			b.Fatalf("place failed: %v", err)
		}
		_ = eng.CancelOrder(res.Order.ID, res.Order.UserID)
		eng.ReleaseResult(res)
	}
}

func BenchmarkPlaceOrderIOC(b *testing.B) {
	eng := newTestEngine()
	_ = eng.SetBalance(1, "USDT", 10_000_000_000)
	_ = eng.SetBalance(2, "BTC", 10_000_000_000)
	b.ReportAllocs()

	maker := &types.OrderInput{
		UserID:   2,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_SPOT,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 10_000_000,
		Price:    50000,
	}
	if _, err := eng.PlaceOrder(maker); err != nil {
		b.Fatalf("maker failed: %v", err)
	}

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_SPOT,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_IOC,
		Quantity: 10,
		Price:    50000,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res, err := eng.PlaceOrder(input)
		if err != nil {
			b.Fatalf("place failed: %v", err)
		}
		eng.ReleaseResult(res)
	}
}

func BenchmarkCancelOrder(b *testing.B) {
	eng := newTestEngine()
	_ = eng.SetBalance(1, "USDT", 1_000_000_000)
	b.ReportAllocs()

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_SPOT,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 10,
		Price:    50000,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res, err := eng.PlaceOrder(input)
		if err != nil {
			b.Fatalf("place failed: %v", err)
		}
		if err := eng.CancelOrder(res.Order.ID, res.Order.UserID); err != nil {
			b.Fatalf("cancel failed: %v", err)
		}
		eng.ReleaseResult(res)
	}
}

func BenchmarkOnPriceTick(b *testing.B) {
	eng := newTestEngine()
	_ = eng.SetBalance(1, "USDT", 1_000_000_000)
	b.ReportAllocs()

	input := &types.OrderInput{
		UserID:       1,
		Symbol:       "BTCUSDT",
		Category:     constants.CATEGORY_LINEAR,
		Side:         constants.ORDER_SIDE_BUY,
		Type:         constants.ORDER_TYPE_LIMIT,
		TIF:          constants.TIF_GTC,
		Quantity:     10,
		Price:        50000,
		TriggerPrice: 48000,
		Leverage:     5,
	}
	if _, err := eng.PlaceOrder(input); err != nil {
		b.Fatalf("place conditional failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		eng.OnPriceTick("BTCUSDT", 47000)
	}
}
