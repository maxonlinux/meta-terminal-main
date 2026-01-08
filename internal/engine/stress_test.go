package engine

import (
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func BenchmarkMatching10000OrdersOnePrice(b *testing.B) {
	eng := newTestEngine()
	eng.AddInstrument("BTCUSDT", constants.CATEGORY_SPOT, 50000)
	for i := 1; i <= 100; i++ {
		_ = eng.SetBalance(types.UserID(i), "USDT", 10_000_000_000)
	}
	_ = eng.SetBalance(999, "BTC", 10_000_000_000)

	for i := 0; i < 10000; i++ {
		input := &types.OrderInput{
			UserID:   types.UserID(i%100 + 1),
			Symbol:   "BTCUSDT",
			Category: constants.CATEGORY_SPOT,
			Side:     constants.ORDER_SIDE_BUY,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Quantity: 1,
			Price:    50000,
		}
		res, _ := eng.PlaceOrder(input)
		eng.ReleaseResult(res)
	}

	sellOrder := &types.OrderInput{
		UserID:   999,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_SPOT,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_MARKET,
		TIF:      constants.TIF_IOC,
		Quantity: 10000,
		Price:    0,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		sellOrder.Quantity = 10000
		res, _ := eng.PlaceOrder(sellOrder)
		eng.ReleaseResult(res)
	}
}

func BenchmarkMatching10000PriceLevels(b *testing.B) {
	eng := newTestEngine()
	eng.AddInstrument("BTCUSDT", constants.CATEGORY_SPOT, 50000)
	for i := 1; i <= 100; i++ {
		_ = eng.SetBalance(types.UserID(i), "USDT", 10_000_000_000)
	}
	_ = eng.SetBalance(999, "BTC", 10_000_000_000)

	for i := 0; i < 10000; i++ {
		input := &types.OrderInput{
			UserID:   types.UserID(i%100 + 1),
			Symbol:   "BTCUSDT",
			Category: constants.CATEGORY_SPOT,
			Side:     constants.ORDER_SIDE_BUY,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Quantity: 1,
			Price:    types.Price(50000 + i),
		}
		res, _ := eng.PlaceOrder(input)
		eng.ReleaseResult(res)
	}

	sellOrder := &types.OrderInput{
		UserID:   999,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_SPOT,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_MARKET,
		TIF:      constants.TIF_IOC,
		Quantity: 5000,
		Price:    0,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		sellOrder.Quantity = 5000
		res, _ := eng.PlaceOrder(sellOrder)
		eng.ReleaseResult(res)
	}
}

func BenchmarkMatchingPartialFills(b *testing.B) {
	eng := newTestEngine()
	eng.AddInstrument("BTCUSDT", constants.CATEGORY_SPOT, 50000)
	for i := 1; i <= 100; i++ {
		_ = eng.SetBalance(types.UserID(i), "USDT", 10_000_000_000)
	}
	_ = eng.SetBalance(999, "BTC", 10_000_000_000)

	for i := 0; i < 1000; i++ {
		input := &types.OrderInput{
			UserID:   types.UserID(i%100 + 1),
			Symbol:   "BTCUSDT",
			Category: constants.CATEGORY_SPOT,
			Side:     constants.ORDER_SIDE_BUY,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Quantity: 10,
			Price:    types.Price(50000 + i%100),
		}
		res, _ := eng.PlaceOrder(input)
		eng.ReleaseResult(res)
	}

	sellOrder := &types.OrderInput{
		UserID:   999,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_SPOT,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_MARKET,
		TIF:      constants.TIF_IOC,
		Quantity: 5000,
		Price:    0,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		sellOrder.Quantity = 5000
		res, _ := eng.PlaceOrder(sellOrder)
		eng.ReleaseResult(res)
	}
}

func BenchmarkCancelFromMiddle(b *testing.B) {
	eng := newTestEngine()
	eng.AddInstrument("BTCUSDT", constants.CATEGORY_SPOT, 50000)
	for i := 1; i <= 100; i++ {
		_ = eng.SetBalance(types.UserID(i), "USDT", 10_000_000_000)
	}

	orderIDs := make([]types.OrderID, 10000)
	for i := 0; i < 10000; i++ {
		input := &types.OrderInput{
			UserID:   types.UserID(i%100 + 1),
			Symbol:   "BTCUSDT",
			Category: constants.CATEGORY_SPOT,
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
		_ = eng.CancelOrder(orderIDs[5000], 0)
	}
}

func BenchmarkCancelFromOnePrice(b *testing.B) {
	eng := newTestEngine()
	eng.AddInstrument("BTCUSDT", constants.CATEGORY_SPOT, 50000)
	for i := 1; i <= 100; i++ {
		_ = eng.SetBalance(types.UserID(i), "USDT", 10_000_000_000)
	}

	orderIDs := make([]types.OrderID, 10000)
	for i := 0; i < 10000; i++ {
		input := &types.OrderInput{
			UserID:   types.UserID(i%100 + 1),
			Symbol:   "BTCUSDT",
			Category: constants.CATEGORY_SPOT,
			Side:     constants.ORDER_SIDE_BUY,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Quantity: 10,
			Price:    50000,
		}
		res, _ := eng.PlaceOrder(input)
		orderIDs[i] = res.Order.ID
		eng.ReleaseResult(res)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = eng.CancelOrder(orderIDs[5000], 0)
	}
}

func BenchmarkGetOrderFromMiddle(b *testing.B) {
	eng := newTestEngine()
	eng.AddInstrument("BTCUSDT", constants.CATEGORY_SPOT, 50000)
	for i := 1; i <= 100; i++ {
		_ = eng.SetBalance(types.UserID(i), "USDT", 10_000_000_000)
	}

	orderIDs := make([]types.OrderID, 10000)
	for i := 0; i < 10000; i++ {
		input := &types.OrderInput{
			UserID:   types.UserID(i%100 + 1),
			Symbol:   "BTCUSDT",
			Category: constants.CATEGORY_SPOT,
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
		_ = eng.orderStore.GetByID(orderIDs[5000])
	}
}

func BenchmarkManyPartialMatches(b *testing.B) {
	eng := newTestEngine()
	eng.AddInstrument("BTCUSDT", constants.CATEGORY_SPOT, 50000)
	for i := 1; i <= 100; i++ {
		_ = eng.SetBalance(types.UserID(i), "USDT", 10_000_000_000)
	}
	_ = eng.SetBalance(999, "BTC", 10_000_000_000)

	for i := 0; i < 1000; i++ {
		input := &types.OrderInput{
			UserID:   types.UserID(i%100 + 1),
			Symbol:   "BTCUSDT",
			Category: constants.CATEGORY_SPOT,
			Side:     constants.ORDER_SIDE_BUY,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Quantity: 1,
			Price:    types.Price(50000 + i),
		}
		res, _ := eng.PlaceOrder(input)
		eng.ReleaseResult(res)
	}

	sellOrder := &types.OrderInput{
		UserID:   999,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_SPOT,
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

func Benchmark10000Orders10000PriceLevels(b *testing.B) {
	eng := newTestEngine()
	eng.AddInstrument("BTCUSDT", constants.CATEGORY_SPOT, 50000)
	for i := 1; i <= 100; i++ {
		_ = eng.SetBalance(types.UserID(i), "USDT", 10_000_000_000)
	}
	_ = eng.SetBalance(999, "BTC", 10_000_000_000)

	for i := 0; i < 10000; i++ {
		input := &types.OrderInput{
			UserID:   types.UserID(i%100 + 1),
			Symbol:   "BTCUSDT",
			Category: constants.CATEGORY_SPOT,
			Side:     constants.ORDER_SIDE_BUY,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Quantity: 1,
			Price:    types.Price(50000 + i),
		}
		res, _ := eng.PlaceOrder(input)
		eng.ReleaseResult(res)
	}

	sellOrder := &types.OrderInput{
		UserID:   999,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_SPOT,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_MARKET,
		TIF:      constants.TIF_IOC,
		Quantity: 10000,
		Price:    0,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		sellOrder.Quantity = 10000
		res, _ := eng.PlaceOrder(sellOrder)
		eng.ReleaseResult(res)
	}
}

func BenchmarkGetUserOrders(b *testing.B) {
	eng := newTestEngine()
	eng.AddInstrument("BTCUSDT", constants.CATEGORY_SPOT, 50000)
	_ = eng.SetBalance(1, "USDT", 10_000_000_000)

	for i := 0; i < 10000; i++ {
		input := &types.OrderInput{
			UserID:   1,
			Symbol:   "BTCUSDT",
			Category: constants.CATEGORY_SPOT,
			Side:     constants.ORDER_SIDE_BUY,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Quantity: 10,
			Price:    types.Price(50000 + i),
		}
		res, _ := eng.PlaceOrder(input)
		eng.ReleaseResult(res)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = eng.OpenOrders(1)
	}
}

func BenchmarkGetOrderBook(b *testing.B) {
	eng := newTestEngine()
	eng.AddInstrument("BTCUSDT", constants.CATEGORY_SPOT, 50000)
	ob := eng.orderbooks.Get("BTCUSDT", constants.CATEGORY_SPOT)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = ob.Depth(constants.ORDER_SIDE_BUY, 10)
	}
}
