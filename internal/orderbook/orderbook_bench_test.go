package orderbook

import (
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func BenchmarkMatchSimple(b *testing.B) {
	b.ReportAllocs()
	orders := make([]*types.Order, 100)
	for n := 0; n < 100; n++ {
		orders[n] = &types.Order{
			ID:       types.OrderID(n + 1),
			UserID:   1,
			Symbol:   "BTCUSDT",
			Category: constants.CATEGORY_LINEAR,
			Side:     constants.ORDER_SIDE_SELL,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Status:   constants.ORDER_STATUS_NEW,
			Price:    100,
			Quantity: 1,
		}
	}
	takerTemplate := types.Order{
		ID:       2000,
		UserID:   2,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Status:   constants.ORDER_STATUS_NEW,
		Price:    100,
		Quantity: 50,
	}
	matchesBuf := make([]types.Match, 0, 64)
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		ob := New()
		for n := 0; n < 100; n++ {
			orders[n].Filled = 0
			orders[n].Quantity = 1
			ob.Add(orders[n])
		}

		taker := takerTemplate
		taker.Filled = 0
		b.StartTimer()
		matchesBuf, _ = ob.MatchInto(&taker, taker.Price, matchesBuf)
	}
}

func BenchmarkAvailableQuantity(b *testing.B) {
	b.ReportAllocs()
	ob := New()
	for n := 0; n < 1000; n++ {
		ob.Add(&types.Order{
			ID:       types.OrderID(n + 1),
			UserID:   1,
			Symbol:   "BTCUSDT",
			Category: constants.CATEGORY_LINEAR,
			Side:     constants.ORDER_SIDE_SELL,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Status:   constants.ORDER_STATUS_NEW,
			Price:    types.Price(100 + n),
			Quantity: 1,
		})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ob.AvailableQuantity(constants.ORDER_SIDE_BUY, 1000, 500)
	}
}
