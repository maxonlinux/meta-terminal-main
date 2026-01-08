package engine

import (
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func BenchmarkMatchOrderWith1000Levels(b *testing.B) {
	eng := newTestEngine()
	eng.AddInstrument("BTCUSDT", constants.CATEGORY_LINEAR, 50000)
	for i := 1; i <= 100; i++ {
		_ = eng.SetBalance(types.UserID(i), "USDT", 10_000_000_000)
	}
	_ = eng.SetBalance(999, "USDT", 10_000_000_000)

	for i := 0; i < 1000; i++ {
		input := &types.OrderInput{
			UserID:   types.UserID(i%100 + 1),
			Symbol:   "BTCUSDT",
			Category: constants.CATEGORY_LINEAR,
			Side:     constants.ORDER_SIDE_BUY,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Quantity: 10,
			Price:    types.Price(50000 + i),
		}
		_, _ = eng.PlaceOrder(input)
	}

	sellOrder := &types.OrderInput{
		UserID:   999,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_MARKET,
		TIF:      constants.TIF_IOC,
		Quantity: 1000,
		Price:    0,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		sellOrder.Quantity = 1000
		res, _ := eng.PlaceOrder(sellOrder)
		eng.ReleaseResult(res)
	}
}

func BenchmarkMatchOrderWith100LevelsSamePrice(b *testing.B) {
	eng := newTestEngine()
	eng.AddInstrument("BTCUSDT", constants.CATEGORY_LINEAR, 50000)
	for i := 1; i <= 100; i++ {
		_ = eng.SetBalance(types.UserID(i), "USDT", 10_000_000_000)
	}
	_ = eng.SetBalance(999, "USDT", 10_000_000_000)

	for i := 0; i < 100; i++ {
		input := &types.OrderInput{
			UserID:   types.UserID(i%100 + 1),
			Symbol:   "BTCUSDT",
			Category: constants.CATEGORY_LINEAR,
			Side:     constants.ORDER_SIDE_BUY,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Quantity: 10,
			Price:    50000,
		}
		_, _ = eng.PlaceOrder(input)
	}

	sellOrder := &types.OrderInput{
		UserID:   999,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_MARKET,
		TIF:      constants.TIF_IOC,
		Quantity: 500,
		Price:    0,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		sellOrder.Quantity = 500
		res, _ := eng.PlaceOrder(sellOrder)
		eng.ReleaseResult(res)
	}
}

func BenchmarkAddOrderToBookWith10000Levels(b *testing.B) {
	eng := newTestEngine()
	eng.AddInstrument("BTCUSDT", constants.CATEGORY_LINEAR, 50000)
	_ = eng.SetBalance(1, "USDT", 10_000_000_000)

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 10,
		Price:    50000,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		input.Price = types.Price(50000 + i%10000)
		res, _ := eng.PlaceOrder(input)
		eng.ReleaseResult(res)
	}
}

func BenchmarkAddOrderToBookSamePrice(b *testing.B) {
	eng := newTestEngine()
	eng.AddInstrument("BTCUSDT", constants.CATEGORY_LINEAR, 50000)
	for i := 1; i <= 100; i++ {
		_ = eng.SetBalance(types.UserID(i), "USDT", 10_000_000_000)
	}

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 10,
		Price:    50000,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		input.UserID = types.UserID(i%100 + 1)
		res, _ := eng.PlaceOrder(input)
		eng.ReleaseResult(res)
	}
}

func BenchmarkCancelOrderFromMiddle(b *testing.B) {
	eng := newTestEngine()
	eng.AddInstrument("BTCUSDT", constants.CATEGORY_LINEAR, 50000)
	for i := 1; i <= 100; i++ {
		_ = eng.SetBalance(types.UserID(i), "USDT", 10_000_000_000)
	}

	orderIDs := make([]types.OrderID, 1000)
	for i := 0; i < 1000; i++ {
		input := &types.OrderInput{
			UserID:   types.UserID(i%100 + 1),
			Symbol:   "BTCUSDT",
			Category: constants.CATEGORY_LINEAR,
			Side:     constants.ORDER_SIDE_BUY,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Quantity: 10,
			Price:    types.Price(50000 + i%1000),
		}
		res, _ := eng.PlaceOrder(input)
		orderIDs[i] = res.Order.ID
		eng.ReleaseResult(res)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = eng.CancelOrder(orderIDs[500], 0)
	}
}
