package orderbook

import (
	"testing"

	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/robaho/fixed"
)

func BenchmarkGetMatchesReuse(b *testing.B) {
	ob := New()
	price := types.Price(fixed.NewI(100, 0))
	for i := 0; i < 1000; i++ {
		order := &types.Order{
			ID:       types.OrderID(i + 1),
			UserID:   types.UserID(1),
			Symbol:   "BTCUSDT",
			Category: constants.CATEGORY_SPOT,
			Side:     constants.ORDER_SIDE_SELL,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Price:    price,
			Quantity: types.Quantity(fixed.NewI(1, 0)),
			Status:   constants.ORDER_STATUS_NEW,
		}
		ob.Add(order)
	}

	taker := &types.Order{
		UserID:   types.UserID(2),
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_SPOT,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_IOC,
		Price:    price,
		Quantity: types.Quantity(fixed.NewI(1, 0)),
		Status:   constants.ORDER_STATUS_NEW,
	}

	buf := make([]types.Match, 0, 16)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf = ob.GetMatches(taker, price, buf)
	}
}
