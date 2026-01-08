package orderbook

import (
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/types"
)

func BenchmarkPlaceOrderGTC(b *testing.B) {
	orderStore := state.NewOrderStore()
	ob := New(constants.CATEGORY_SPOT, orderStore)

	order := &types.Order{
		ID:       types.OrderID(1),
		UserID:   1,
		Symbol:   "BTCUSDT",
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Price:    types.Price(50000),
		Quantity: types.Quantity(1),
		Status:   constants.ORDER_STATUS_NEW,
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ob.AddOrder(order)
	}
}

func BenchmarkPlaceOrderIOC(b *testing.B) {
	orderStore := state.NewOrderStore()
	ob := New(constants.CATEGORY_SPOT, orderStore)

	order := &types.Order{
		ID:       types.OrderID(1),
		UserID:   1,
		Symbol:   "BTCUSDT",
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_MARKET,
		TIF:      constants.TIF_IOC,
		Price:    types.Price(50000),
		Quantity: types.Quantity(1),
		Status:   constants.ORDER_STATUS_NEW,
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ob.AddOrder(order)
	}
}

func BenchmarkMatchOrder(b *testing.B) {
	orderStore := state.NewOrderStore()
	ob := New(constants.CATEGORY_SPOT, orderStore)

	for i := 0; i < 100; i++ {
		maker := &types.Order{
			ID:       types.OrderID(i + 1),
			UserID:   types.UserID(i + 2),
			Symbol:   "BTCUSDT",
			Side:     constants.ORDER_SIDE_SELL,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Price:    types.Price(50000 + int64(i)),
			Quantity: types.Quantity(10),
			Status:   constants.ORDER_STATUS_NEW,
		}
		orderStore.Add(maker)
		ob.AddOrder(maker)
	}

	taker := &types.Order{
		ID:       types.OrderID(999),
		UserID:   1,
		Symbol:   "BTCUSDT",
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_MARKET,
		TIF:      constants.TIF_IOC,
		Price:    types.Price(51000),
		Quantity: types.Quantity(100),
		Status:   constants.ORDER_STATUS_NEW,
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ob.AddOrder(taker)
	}
}

func BenchmarkGetBestBid(b *testing.B) {
	orderStore := state.NewOrderStore()
	ob := New(constants.CATEGORY_SPOT, orderStore)

	for i := 0; i < 100; i++ {
		order := &types.Order{
			ID:       types.OrderID(i + 1),
			UserID:   types.UserID(i + 2),
			Symbol:   "BTCUSDT",
			Side:     constants.ORDER_SIDE_BUY,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Price:    types.Price(50000 - int64(i)),
			Quantity: types.Quantity(10),
			Status:   constants.ORDER_STATUS_NEW,
		}
		orderStore.Add(order)
		ob.AddOrder(order)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ob.GetBestBid()
	}
}

func BenchmarkGetBestAsk(b *testing.B) {
	orderStore := state.NewOrderStore()
	ob := New(constants.CATEGORY_SPOT, orderStore)

	for i := 0; i < 100; i++ {
		order := &types.Order{
			ID:       types.OrderID(i + 1),
			UserID:   types.UserID(i + 2),
			Symbol:   "BTCUSDT",
			Side:     constants.ORDER_SIDE_SELL,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Price:    types.Price(50000 + int64(i)),
			Quantity: types.Quantity(10),
			Status:   constants.ORDER_STATUS_NEW,
		}
		orderStore.Add(order)
		ob.AddOrder(order)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ob.GetBestAsk()
	}
}

func BenchmarkGetDepth(b *testing.B) {
	orderStore := state.NewOrderStore()
	ob := New(constants.CATEGORY_SPOT, orderStore)

	for i := 0; i < 100; i++ {
		bid := &types.Order{
			ID:       types.OrderID(i + 1),
			UserID:   types.UserID(i + 2),
			Symbol:   "BTCUSDT",
			Side:     constants.ORDER_SIDE_BUY,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Price:    types.Price(50000 - int64(i)),
			Quantity: types.Quantity(10),
			Status:   constants.ORDER_STATUS_NEW,
		}
		orderStore.Add(bid)
		ob.AddOrder(bid)

		ask := &types.Order{
			ID:       types.OrderID(i + 101),
			UserID:   types.UserID(i + 102),
			Symbol:   "BTCUSDT",
			Side:     constants.ORDER_SIDE_SELL,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Price:    types.Price(50100 + int64(i)),
			Quantity: types.Quantity(10),
			Status:   constants.ORDER_STATUS_NEW,
		}
		orderStore.Add(ask)
		ob.AddOrder(ask)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ob.GetDepth(constants.ORDER_SIDE_BUY, 10)
		ob.GetDepth(constants.ORDER_SIDE_SELL, 10)
	}
}

func BenchmarkRemoveOrder(b *testing.B) {
	orderStore := state.NewOrderStore()
	ob := New(constants.CATEGORY_SPOT, orderStore)

	order := &types.Order{
		ID:       types.OrderID(1),
		UserID:   1,
		Symbol:   "BTCUSDT",
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Price:    types.Price(50000),
		Quantity: types.Quantity(10),
		Status:   constants.ORDER_STATUS_NEW,
	}
	orderStore.Add(order)
	ob.AddOrder(order)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ob.RemoveOrder(order)
	}
}

func BenchmarkWouldCross(b *testing.B) {
	orderStore := state.NewOrderStore()
	ob := New(constants.CATEGORY_SPOT, orderStore)

	for i := 0; i < 100; i++ {
		ask := &types.Order{
			ID:       types.OrderID(i + 1),
			UserID:   types.UserID(i + 2),
			Symbol:   "BTCUSDT",
			Side:     constants.ORDER_SIDE_SELL,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Price:    types.Price(50100 + int64(i)),
			Quantity: types.Quantity(10),
			Status:   constants.ORDER_STATUS_NEW,
		}
		orderStore.Add(ask)
		ob.AddOrder(ask)
	}

	order := &types.Order{
		ID:       types.OrderID(999),
		UserID:   1,
		Symbol:   "BTCUSDT",
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_POST_ONLY,
		Price:    types.Price(50200),
		Quantity: types.Quantity(10),
		Status:   constants.ORDER_STATUS_NEW,
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ob.WouldCross(order)
	}
}

func BenchmarkAvailableQuantity(b *testing.B) {
	orderStore := state.NewOrderStore()
	ob := New(constants.CATEGORY_SPOT, orderStore)

	for i := 0; i < 100; i++ {
		ask := &types.Order{
			ID:       types.OrderID(i + 1),
			UserID:   types.UserID(i + 2),
			Symbol:   "BTCUSDT",
			Side:     constants.ORDER_SIDE_SELL,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Price:    types.Price(50100 + int64(i)),
			Quantity: types.Quantity(10),
			Status:   constants.ORDER_STATUS_NEW,
		}
		orderStore.Add(ask)
		ob.AddOrder(ask)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ob.AvailableQuantity(constants.ORDER_SIDE_BUY, 50500, types.Quantity(1000))
	}
}
