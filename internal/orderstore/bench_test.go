package orderstore

import (
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func BenchmarkStore(b *testing.B) {
	store := New()
	orders := make([]*types.Order, 1000)
	for i := 0; i < 1000; i++ {
		orders[i] = &types.Order{
			ID:       types.OrderID(i + 1),
			UserID:   types.UserID(i % 100),
			Symbol:   "BTCUSDT",
			Side:     int8(i % 2),
			Type:     0,
			TIF:      0,
			Status:   0,
			Price:    types.Price(50000 + i%1000),
			Quantity: 10,
			Filled:   0,
		}
	}

	b.ResetTimer()
	b.Run("AddOrder", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			store.Add(orders[i%1000])
		}
	})

	b.Run("GetOrder", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			store.Get(types.UserID(i%100), types.OrderID(i%1000+1))
		}
	})

	b.Run("RemoveOrder", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			store.Remove(types.UserID(i%100), types.OrderID(i%1000+1))
		}
	})
}
