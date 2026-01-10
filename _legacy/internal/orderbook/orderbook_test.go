package orderbook

import (
	"sync/atomic"
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

type seqGen struct {
	seq uint64
}

func (g *seqGen) Next() uint64 {
	return atomic.AddUint64(&g.seq, 1)
}

func BenchmarkOrderBook_PlaceOrder(b *testing.B) {
	ob := NewWithIDGenerator(&seqGen{})

	for i := 0; i < 1000; i++ {
		order := &types.Order{
			ID:        types.OrderID(i + 1),
			UserID:    types.UserID(i + 1),
			Symbol:    "BTCUSDT",
			Category:  constants.CATEGORY_LINEAR,
			Side:      constants.ORDER_SIDE_SELL,
			Type:      constants.ORDER_TYPE_LIMIT,
			TIF:       constants.TIF_GTC,
			Quantity:  types.Quantity(100),
			Price:     types.Price(50000 + i),
			Status:    constants.ORDER_STATUS_NEW,
			CreatedAt: types.NowNano(),
		}
		ob.AddResting(order)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		order := &types.Order{
			ID:        types.OrderID(1000000 + i),
			UserID:    types.UserID(1000000 + i),
			Symbol:    "BTCUSDT",
			Category:  constants.CATEGORY_LINEAR,
			Side:      constants.ORDER_SIDE_BUY,
			Type:      constants.ORDER_TYPE_LIMIT,
			TIF:       constants.TIF_GTC,
			Quantity:  types.Quantity(100),
			Price:     types.Price(50000 + i%1000),
			Status:    constants.ORDER_STATUS_NEW,
			CreatedAt: types.NowNano(),
		}
		ob.AddResting(order)
	}
}

func BenchmarkOrderBook_MatchOrder(b *testing.B) {
	ob := NewWithIDGenerator(&seqGen{})

	for i := 0; i < 1000; i++ {
		order := &types.Order{
			ID:        types.OrderID(i + 1),
			UserID:    types.UserID(i + 1),
			Symbol:    "BTCUSDT",
			Category:  constants.CATEGORY_LINEAR,
			Side:      constants.ORDER_SIDE_SELL,
			Type:      constants.ORDER_TYPE_LIMIT,
			TIF:       constants.TIF_GTC,
			Quantity:  types.Quantity(100),
			Price:     types.Price(50000 + i),
			Status:    constants.ORDER_STATUS_NEW,
			CreatedAt: types.NowNano(),
		}
		ob.AddResting(order)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		taker := &types.Order{
			ID:        types.OrderID(1000000 + i),
			UserID:    types.UserID(1000000 + i),
			Symbol:    "BTCUSDT",
			Category:  constants.CATEGORY_LINEAR,
			Side:      constants.ORDER_SIDE_BUY,
			Type:      constants.ORDER_TYPE_LIMIT,
			TIF:       constants.TIF_GTC,
			Quantity:  types.Quantity(100),
			Price:     types.Price(50500),
			Status:    constants.ORDER_STATUS_NEW,
			CreatedAt: types.NowNano(),
		}
		ob.Match(taker, types.Price(50500))
	}
}

func BenchmarkOrderBook_MatchOrder_Market(b *testing.B) {
	ob := NewWithIDGenerator(&seqGen{})

	for i := 0; i < 1000; i++ {
		order := &types.Order{
			ID:        types.OrderID(i + 1),
			UserID:    types.UserID(i + 1),
			Symbol:    "BTCUSDT",
			Category:  constants.CATEGORY_LINEAR,
			Side:      constants.ORDER_SIDE_SELL,
			Type:      constants.ORDER_TYPE_LIMIT,
			TIF:       constants.TIF_GTC,
			Quantity:  types.Quantity(100),
			Price:     types.Price(50000 + i),
			Status:    constants.ORDER_STATUS_NEW,
			CreatedAt: types.NowNano(),
		}
		ob.AddResting(order)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		taker := &types.Order{
			ID:        types.OrderID(1000000 + i),
			UserID:    types.UserID(1000000 + i),
			Symbol:    "BTCUSDT",
			Category:  constants.CATEGORY_LINEAR,
			Side:      constants.ORDER_SIDE_BUY,
			Type:      constants.ORDER_TYPE_MARKET,
			TIF:       constants.TIF_IOC,
			Quantity:  types.Quantity(1000),
			Price:     types.Price(0),
			Status:    constants.ORDER_STATUS_NEW,
			CreatedAt: types.NowNano(),
		}
		ob.Match(taker, types.Price(0))
	}
}

func BenchmarkOrderBook_BestBidAsk(b *testing.B) {
	ob := NewWithIDGenerator(&seqGen{})

	for i := 0; i < 1000; i++ {
		ob.AddResting(&types.Order{
			ID:        types.OrderID(i + 1),
			UserID:    types.UserID(i + 1),
			Symbol:    "BTCUSDT",
			Category:  constants.CATEGORY_LINEAR,
			Side:      constants.ORDER_SIDE_BUY,
			Type:      constants.ORDER_TYPE_LIMIT,
			TIF:       constants.TIF_GTC,
			Quantity:  types.Quantity(100),
			Price:     types.Price(50000 - i%100),
			Status:    constants.ORDER_STATUS_NEW,
			CreatedAt: types.NowNano(),
		})
		ob.AddResting(&types.Order{
			ID:        types.OrderID(i + 1001),
			UserID:    types.UserID(i + 1001),
			Symbol:    "BTCUSDT",
			Category:  constants.CATEGORY_LINEAR,
			Side:      constants.ORDER_SIDE_SELL,
			Type:      constants.ORDER_TYPE_LIMIT,
			TIF:       constants.TIF_GTC,
			Quantity:  types.Quantity(100),
			Price:     types.Price(50100 + i%100),
			Status:    constants.ORDER_STATUS_NEW,
			CreatedAt: types.NowNano(),
		})
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ob.BestBid()
		ob.BestAsk()
	}
}

func BenchmarkOrderBook_Depth(b *testing.B) {
	ob := NewWithIDGenerator(&seqGen{})

	for i := 0; i < 1000; i++ {
		ob.AddResting(&types.Order{
			ID:        types.OrderID(i + 1),
			UserID:    types.UserID(i + 1),
			Symbol:    "BTCUSDT",
			Category:  constants.CATEGORY_LINEAR,
			Side:      constants.ORDER_SIDE_BUY,
			Type:      constants.ORDER_TYPE_LIMIT,
			TIF:       constants.TIF_GTC,
			Quantity:  types.Quantity(100),
			Price:     types.Price(50000 - i%100),
			Status:    constants.ORDER_STATUS_NEW,
			CreatedAt: types.NowNano(),
		})
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ob.Depth(constants.ORDER_SIDE_BUY, 10)
	}
}

func BenchmarkOrderBook_CancelOrder(b *testing.B) {
	ob := NewWithIDGenerator(&seqGen{})

	orders := make([]types.OrderID, 1000)
	for i := 0; i < 1000; i++ {
		orderID := types.OrderID(i + 1)
		orders[i] = orderID
		ob.AddResting(&types.Order{
			ID:        orderID,
			UserID:    types.UserID(i + 1),
			Symbol:    "BTCUSDT",
			Category:  constants.CATEGORY_LINEAR,
			Side:      constants.ORDER_SIDE_BUY,
			Type:      constants.ORDER_TYPE_LIMIT,
			TIF:       constants.TIF_GTC,
			Quantity:  types.Quantity(100),
			Price:     types.Price(50000 - i%100),
			Status:    constants.ORDER_STATUS_NEW,
			CreatedAt: types.NowNano(),
		})
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ob.RemoveResting(orders[i%1000])
	}
}

func BenchmarkOrderBook_ConcurrentMatch(b *testing.B) {
	ob := NewWithIDGenerator(&seqGen{})

	for i := 0; i < 5000; i++ {
		ob.AddResting(&types.Order{
			ID:        types.OrderID(i + 1),
			UserID:    types.UserID(i + 1),
			Symbol:    "BTCUSDT",
			Category:  constants.CATEGORY_LINEAR,
			Side:      constants.ORDER_SIDE_SELL,
			Type:      constants.ORDER_TYPE_LIMIT,
			TIF:       constants.TIF_GTC,
			Quantity:  types.Quantity(100),
			Price:     types.Price(50000 + i%1000),
			Status:    constants.ORDER_STATUS_NEW,
			CreatedAt: types.NowNano(),
		})
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := uint64(0)
		for pb.Next() {
			taker := &types.Order{
				ID:        types.OrderID(1000000 + i),
				UserID:    types.UserID(1000000 + i),
				Symbol:    "BTCUSDT",
				Category:  constants.CATEGORY_LINEAR,
				Side:      constants.ORDER_SIDE_BUY,
				Type:      constants.ORDER_TYPE_LIMIT,
				TIF:       constants.TIF_GTC,
				Quantity:  types.Quantity(100),
				Price:     types.Price(50500),
				Status:    constants.ORDER_STATUS_NEW,
				CreatedAt: types.NowNano(),
			}
			ob.Match(taker, types.Price(50500))
			i++
		}
	})
}
