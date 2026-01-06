package engine

import (
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/internal/types"
	"github.com/anomalyco/meta-terminal-go/internal/wal"
)

func BenchmarkMatching10000OrdersOnePrice(b *testing.B) {
	s := state.New()
	w, _ := wal.New("/tmp/wal_test", 64)
	defer w.Close()

	e := New(w, s)
	e.InitSymbolCategory(1, constants.CATEGORY_SPOT)

	for i := range 10000 {
		input := &types.OrderInput{
			UserID:   types.UserID(i%100 + 1),
			Symbol:   types.SymbolID(1),
			Side:     constants.ORDER_SIDE_BUY,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Quantity: types.Quantity(1),
			Price:    types.Price(50000),
		}
		e.PlaceOrder(input)
	}

	sellOrder := &types.OrderInput{
		UserID:   types.UserID(999),
		Symbol:   types.SymbolID(1),
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_MARKET,
		TIF:      constants.TIF_IOC,
		Quantity: types.Quantity(10000),
		Price:    0,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		sellOrder.Quantity = types.Quantity(10000)
		e.PlaceOrder(sellOrder)
	}
}

func BenchmarkMatching10000PriceLevels(b *testing.B) {
	s := state.New()
	w, _ := wal.New("/tmp/wal_test", 64)
	defer w.Close()

	e := New(w, s)
	e.InitSymbolCategory(1, constants.CATEGORY_SPOT)

	for i := 0; i < 10000; i++ {
		input := &types.OrderInput{
			UserID:   types.UserID(i%100 + 1),
			Symbol:   types.SymbolID(1),
			Side:     constants.ORDER_SIDE_BUY,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Quantity: types.Quantity(1),
			Price:    types.Price(50000 + i),
		}
		e.PlaceOrder(input)
	}

	sellOrder := &types.OrderInput{
		UserID:   types.UserID(999),
		Symbol:   types.SymbolID(1),
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_MARKET,
		TIF:      constants.TIF_IOC,
		Quantity: types.Quantity(5000),
		Price:    0,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		sellOrder.Quantity = types.Quantity(5000)
		e.PlaceOrder(sellOrder)
	}
}

func BenchmarkMatchingPartialFills(b *testing.B) {
	s := state.New()
	w, _ := wal.New("/tmp/wal_test", 64)
	defer w.Close()

	e := New(w, s)
	e.InitSymbolCategory(1, constants.CATEGORY_SPOT)

	for i := 0; i < 1000; i++ {
		input := &types.OrderInput{
			UserID:   types.UserID(i%100 + 1),
			Symbol:   types.SymbolID(1),
			Side:     constants.ORDER_SIDE_BUY,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Quantity: types.Quantity(10),
			Price:    types.Price(50000 + i%100),
		}
		e.PlaceOrder(input)
	}

	sellOrder := &types.OrderInput{
		UserID:   types.UserID(999),
		Symbol:   types.SymbolID(1),
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_MARKET,
		TIF:      constants.TIF_IOC,
		Quantity: types.Quantity(5000),
		Price:    0,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		sellOrder.Quantity = types.Quantity(5000)
		e.PlaceOrder(sellOrder)
	}
}

func BenchmarkCancelFromMiddle(b *testing.B) {
	s := state.New()
	w, _ := wal.New("/tmp/wal_test", 64)
	defer w.Close()

	e := New(w, s)
	e.InitSymbolCategory(1, constants.CATEGORY_SPOT)

	var orderIDs []types.OrderID
	for i := 0; i < 10000; i++ {
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
		orderIDs = append(orderIDs, result.Order.ID)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		e.CancelOrder(orderIDs[5000], 0)
	}
}

func BenchmarkCancelFromOnePrice(b *testing.B) {
	s := state.New()
	w, _ := wal.New("/tmp/wal_test", 64)
	defer w.Close()

	e := New(w, s)
	e.InitSymbolCategory(1, constants.CATEGORY_SPOT)

	var orderIDs []types.OrderID
	for i := 0; i < 10000; i++ {
		input := &types.OrderInput{
			UserID:   types.UserID(i%100 + 1),
			Symbol:   types.SymbolID(1),
			Side:     constants.ORDER_SIDE_BUY,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Quantity: types.Quantity(10),
			Price:    types.Price(50000),
		}
		result, _ := e.PlaceOrder(input)
		orderIDs = append(orderIDs, result.Order.ID)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		e.CancelOrder(orderIDs[5000], 0)
	}
}

func BenchmarkGetOrderFromMiddle(b *testing.B) {
	s := state.New()
	w, _ := wal.New("/tmp/wal_test", 64)
	defer w.Close()

	e := New(w, s)
	e.InitSymbolCategory(1, constants.CATEGORY_SPOT)

	var orderIDs []types.OrderID
	for i := 0; i < 10000; i++ {
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
		orderIDs = append(orderIDs, result.Order.ID)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		e.GetOrder(orderIDs[5000])
	}
}

func BenchmarkManyPartialMatches(b *testing.B) {
	s := state.New()
	w, _ := wal.New("/tmp/wal_test", 64)
	defer w.Close()

	e := New(w, s)
	e.InitSymbolCategory(1, constants.CATEGORY_SPOT)

	for i := 0; i < 1000; i++ {
		input := &types.OrderInput{
			UserID:   types.UserID(i%100 + 1),
			Symbol:   types.SymbolID(1),
			Side:     constants.ORDER_SIDE_BUY,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Quantity: types.Quantity(1),
			Price:    types.Price(50000 + i),
		}
		e.PlaceOrder(input)
	}

	sellOrder := &types.OrderInput{
		UserID:   types.UserID(999),
		Symbol:   types.SymbolID(1),
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_MARKET,
		TIF:      constants.TIF_IOC,
		Quantity: types.Quantity(1000),
		Price:    0,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		sellOrder.Quantity = types.Quantity(1000)
		e.PlaceOrder(sellOrder)
	}
}

func Benchmark10000Orders10000PriceLevels(b *testing.B) {
	s := state.New()
	w, _ := wal.New("/tmp/wal_test", 64)
	defer w.Close()

	e := New(w, s)
	e.InitSymbolCategory(1, constants.CATEGORY_SPOT)

	for i := 0; i < 10000; i++ {
		input := &types.OrderInput{
			UserID:   types.UserID(i%100 + 1),
			Symbol:   types.SymbolID(1),
			Side:     constants.ORDER_SIDE_BUY,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Quantity: types.Quantity(1),
			Price:    types.Price(50000 + i),
		}
		e.PlaceOrder(input)
	}

	sellOrder := &types.OrderInput{
		UserID:   types.UserID(999),
		Symbol:   types.SymbolID(1),
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_MARKET,
		TIF:      constants.TIF_IOC,
		Quantity: types.Quantity(10000),
		Price:    0,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		sellOrder.Quantity = types.Quantity(10000)
		e.PlaceOrder(sellOrder)
	}
}

func BenchmarkGetUserOrders(b *testing.B) {
	s := state.New()
	w, _ := wal.New("/tmp/wal_test", 64)
	defer w.Close()

	e := New(w, s)
	e.InitSymbolCategory(1, constants.CATEGORY_SPOT)

	for i := 0; i < 10000; i++ {
		input := &types.OrderInput{
			UserID:   types.UserID(1),
			Symbol:   types.SymbolID(1),
			Side:     constants.ORDER_SIDE_BUY,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Quantity: types.Quantity(10),
			Price:    types.Price(50000 + i),
		}
		e.PlaceOrder(input)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		e.GetUserOrders(1)
	}
}

func BenchmarkGetSymbolState(b *testing.B) {
	s := state.New()
	w, _ := wal.New("/tmp/wal_test", 64)
	defer w.Close()

	e := New(w, s)
	e.InitSymbolCategory(1, constants.CATEGORY_SPOT)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		e.state.GetSymbolState(1)
	}
}
