package orderbook

import (
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func BenchmarkMatch(b *testing.B) {
	ob := New()
	b.ReportAllocs()
	for i := 0; i < 100; i++ {
		order := &types.Order{
			ID:       types.OrderID(i + 1),
			UserID:   1,
			Symbol:   "BTCUSDT",
			Category: constants.CATEGORY_SPOT,
			Side:     constants.ORDER_SIDE_SELL,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Price:    types.Price(50000 + i),
			Quantity: 100,
		}
		ob.AddResting(order)
	}

	taker := &types.Order{
		ID:       9999,
		UserID:   2,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_SPOT,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_IOC,
		Price:    60000,
		Quantity: 50,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		taker.Filled = 0
		_, _ = ob.Match(taker, taker.Price)
	}
}
