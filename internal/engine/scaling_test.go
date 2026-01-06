package engine

import (
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/internal/types"
	"github.com/anomalyco/meta-terminal-go/internal/wal"
)

func BenchmarkMatchOrderWith1000Levels(b *testing.B) {
	s := state.New()
	w, _ := wal.New("/tmp/wal_test", 64)
	defer w.Close()

	e := New(w, s)
	e.InitSymbolCategory(1, constants.CATEGORY_LINEAR)

	for i := 0; i < 1000; i++ {
		input := &types.OrderInput{
			UserID:   types.UserID(i%100 + 1),
			Symbol:   types.SymbolID(1),
			Side:     constants.ORDER_SIDE_BUY,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Quantity: types.Quantity(10),
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

func BenchmarkMatchOrderWith100LevelsSamePrice(b *testing.B) {
	s := state.New()
	w, _ := wal.New("/tmp/wal_test", 64)
	defer w.Close()

	e := New(w, s)
	e.InitSymbolCategory(1, constants.CATEGORY_LINEAR)

	for i := 0; i < 100; i++ {
		input := &types.OrderInput{
			UserID:   types.UserID(i%100 + 1),
			Symbol:   types.SymbolID(1),
			Side:     constants.ORDER_SIDE_BUY,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Quantity: types.Quantity(10),
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
		Quantity: types.Quantity(500),
		Price:    0,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		sellOrder.Quantity = types.Quantity(500)
		e.PlaceOrder(sellOrder)
	}
}

func BenchmarkAddOrderToBookWith10000Levels(b *testing.B) {
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
		input.Price = types.Price(50000 + i%10000)
		e.PlaceOrder(input)
	}
}

func BenchmarkAddOrderToBookSamePrice(b *testing.B) {
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
		e.PlaceOrder(input)
	}
}

func BenchmarkCancelOrderFromMiddle(b *testing.B) {
	s := state.New()
	w, _ := wal.New("/tmp/wal_test", 64)
	defer w.Close()

	e := New(w, s)
	e.InitSymbolCategory(1, constants.CATEGORY_LINEAR)

	var orderIDs []types.OrderID
	for i := 0; i < 1000; i++ {
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
		e.CancelOrder(orderIDs[500], 0)
	}
}
