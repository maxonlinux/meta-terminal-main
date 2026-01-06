package engine

import (
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/internal/types"
	"github.com/anomalyco/meta-terminal-go/internal/wal"
)

// BenchmarkPlaceOrder benchmarks the PlaceOrder operation
func BenchmarkPlaceOrder(b *testing.B) {
	s := state.New()
	w, _ := wal.New("/tmp/wal_test", 64)
	defer w.Close()

	e := New(w, s)
	e.InitSymbolCategory(1, constants.CATEGORY_LINEAR)

	input := &types.OrderInput{
		UserID:   types.UserID(1),
		Symbol:   types.SymbolID(1),
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: types.Quantity(10),
		Price:    types.Price(50000),
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		input.UserID = types.UserID(i%100 + 1)
		input.Price = types.Price(50000 + i%1000)
		result, _ := e.PlaceOrder(input)
		e.CancelOrder(result.Order.ID, 0)
	}
}

// BenchmarkCancelOrder benchmarks order cancellation
func BenchmarkCancelOrder(b *testing.B) {
	s := state.New()
	w, _ := wal.New("/tmp/wal_cancel", 64)
	defer w.Close()

	e := New(w, s)
	e.InitSymbolCategory(1, constants.CATEGORY_LINEAR)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		input := &types.OrderInput{
			UserID:   types.UserID(1),
			Symbol:   types.SymbolID(1),
			Side:     constants.ORDER_SIDE_BUY,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Quantity: types.Quantity(10),
			Price:    types.Price(50000 + i%1000),
		}
		result, _ := e.PlaceOrder(input)
		e.CancelOrder(result.Order.ID, 0)
	}
}

// BenchmarkAmendOrder benchmarks order amendment
func BenchmarkAmendOrder(b *testing.B) {
	s := state.New()
	w, _ := wal.New("/tmp/wal_amend", 64)
	defer w.Close()

	e := New(w, s)
	e.InitSymbolCategory(1, constants.CATEGORY_LINEAR)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		input := &types.OrderInput{
			UserID:   types.UserID(1),
			Symbol:   types.SymbolID(1),
			Side:     constants.ORDER_SIDE_BUY,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Quantity: types.Quantity(10),
			Price:    types.Price(50000 + i%1000),
		}
		result, _ := e.PlaceOrder(input)
		e.AmendOrder(result.Order.ID, 0, types.Quantity(15), types.Price(50100+i%1000))
	}
}

// BenchmarkGetOrder benchmarks order retrieval
func BenchmarkGetOrder(b *testing.B) {
	s := state.New()
	w, _ := wal.New("/tmp/wal_test4", 64)
	defer w.Close()

	e := New(w, s)
	e.InitSymbolCategory(1, constants.CATEGORY_LINEAR)

	// Place orders first
	orderIDs := make([]types.OrderID, 1000)
	for i := 0; i < 1000; i++ {
		input := &types.OrderInput{
			UserID:   types.UserID(1),
			Symbol:   types.SymbolID(1),
			Side:     constants.ORDER_SIDE_BUY,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Quantity: types.Quantity(10),
			Price:    types.Price(50000 + i),
		}
		result, _ := e.PlaceOrder(input)
		orderIDs[i] = result.Order.ID
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		e.GetOrder(orderIDs[i%1000])
	}
}

// BenchmarkEndToEnd benchmarks complete order flow
func BenchmarkEndToEnd(b *testing.B) {
	s := state.New()
	w, _ := wal.New("/tmp/wal_test5", 64)
	defer w.Close()

	e := New(w, s)
	e.InitSymbolCategory(1, constants.CATEGORY_LINEAR)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Place order
		input := &types.OrderInput{
			UserID:   types.UserID(i%100 + 1),
			Symbol:   types.SymbolID(1),
			Side:     constants.ORDER_SIDE_BUY,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Quantity: types.Quantity(10),
			Price:    types.Price(50000 + i%1000),
		}
		result, _ := e.PlaceOrder(input)

		// Cancel order
		e.CancelOrder(result.Order.ID, 0)
	}
}

// BenchmarkGetDepth benchmarks getting order book depth for WebSocket
// Returns: b (bids array), a (asks array), ts (timestamp ms)
func BenchmarkGetDepth(b *testing.B) {
	s := state.New()
	w, _ := wal.New("/tmp/wal_test6", 64)
	defer w.Close()

	e := New(w, s)
	e.InitSymbolCategory(1, constants.CATEGORY_LINEAR)

	// Create order book with many levels
	for i := 0; i < 1000; i++ {
		// Add bid levels from 49900 to 49999
		input := &types.OrderInput{
			UserID:   types.UserID(i%100 + 1),
			Symbol:   types.SymbolID(1),
			Side:     constants.ORDER_SIDE_BUY,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Quantity: types.Quantity(10),
			Price:    types.Price(49900 + i%100),
		}
		e.PlaceOrder(input)

		// Add ask levels from 50100 to 50199
		input2 := &types.OrderInput{
			UserID:   types.UserID(i%100 + 101),
			Symbol:   types.SymbolID(1),
			Side:     constants.ORDER_SIDE_SELL,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Quantity: types.Quantity(10),
			Price:    types.Price(50100 + i%100),
		}
		e.PlaceOrder(input2)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Get depth via public API (includes flattenDepth)
		bids, asks := e.GetOrderBook(1, 10)
		_ = bids
		_ = asks
	}
}
