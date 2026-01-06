package memory

import (
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/types"
)

// BenchmarkOrderStore benchmarks the OrderStore performance
func BenchmarkOrderStore(b *testing.B) {
	os := NewOrderStore()

	// Pre-create some orders
	orders := make([]*types.Order, 1000)
	for i := 0; i < 1000; i++ {
		orders[i] = &types.Order{
			ID:       types.OrderID(i + 1),
			UserID:   types.UserID(i % 100),
			Symbol:   types.SymbolID(i % 100),
			Side:     int8(i % 2),
			Type:     0,
			TIF:      0,
			Status:   0,
			Price:    types.Price(50000 + i%1000),
			Quantity: types.Quantity(10),
			Filled:   0,
		}
	}

	b.ResetTimer()
	b.Run("AddOrder", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			os.Add(orders[i%1000])
		}
	})

	b.Run("GetOrder", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			os.Get(types.OrderID(i%1000 + 1))
		}
	})

	b.Run("RemoveOrder", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			os.Remove(types.OrderID(i%1000 + 1))
		}
	})
}

// BenchmarkPool benchmarks the order pool performance
func BenchmarkPool(b *testing.B) {
	pool := NewOrderPool()

	b.ResetTimer()
	b.Run("GetPut", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			order := pool.Get()
			order.ID = types.OrderID(i)
			order.UserID = types.UserID(i % 100)
			order.Symbol = types.SymbolID(i % 100)
			order.Price = types.Price(50000)
			order.Quantity = types.Quantity(10)
			pool.Put(order)
		}
	})
}

// BenchmarkTradeBuffer benchmarks the trade buffer performance
func BenchmarkTradeBuffer(b *testing.B) {
	buffer := NewTradeBuffer(GetTradePool(), 64)
	trades := make([]*types.Trade, 64)

	for i := 0; i < 64; i++ {
		trades[i] = &types.Trade{
			ID:           types.OrderID(i),
			Symbol:       types.SymbolID(1),
			BuyerID:      types.UserID(1),
			SellerID:     types.UserID(2),
			Price:        types.Price(50000),
			Quantity:     types.Quantity(10),
			TakerOrderID: types.OrderID(i),
			MakerOrderID: types.OrderID(i + 1),
		}
	}

	b.ResetTimer()
	b.Run("AddSlice", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			buffer.Reset()
			for j := 0; j < 64; j++ {
				buffer.Add(trades[j])
			}
		}
	})
}
