package engine

import (
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/maxonlinux/meta-terminal-go/internal/oms"
	"github.com/maxonlinux/meta-terminal-go/internal/registry"
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/robaho/fixed"
)

func BenchmarkEngine_Throughput_Compare(b *testing.B) {
	reg := registry.New()
	reg.SetInstrument("BTCUSDT", registry.FromSymbol("BTCUSDT", 50000))

	store := oms.NewService(nil)
	cb := &mockCallback{}
	e := NewEngine(store, reg, cb)

	for i := 0; i < 1000; i++ {
		userID := types.UserID(i + 1)
		e.portfolio.Balances[userID] = map[string]*types.Balance{
			"USDT": {UserID: userID, Asset: "USDT", Available: types.Quantity(fixed.NewI(100000000, 0))},
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		userID := types.UserID(i%1000 + 1)
		result := e.Cmd(&PlaceOrderCmd{
			Req: &types.PlaceOrderRequest{
				UserID:   userID,
				Symbol:   "BTCUSDT",
				Category: constants.CATEGORY_SPOT,
				Side:     constants.ORDER_SIDE_BUY,
				Type:     constants.ORDER_TYPE_LIMIT,
				TIF:      constants.TIF_GTC,
				Quantity: types.Quantity(fixed.NewI(1, 0)),
				Price:    types.Price(fixed.NewI(50000, 0)),
			},
		})
		_ = result.Order
		_ = result.Err
	}

	e.Shutdown()
}

func BenchmarkEngine_Throughput_WithTrace(b *testing.B) {
	reg := registry.New()
	reg.SetInstrument("BTCUSDT", registry.FromSymbol("BTCUSDT", 50000))

	store := oms.NewService(nil)
	cb := &mockCallback{}
	e := NewEngine(store, reg, cb)

	for i := 0; i < 1000; i++ {
		userID := types.UserID(i + 1)
		e.portfolio.Balances[userID] = map[string]*types.Balance{
			"USDT": {UserID: userID, Asset: "USDT", Available: types.Quantity(fixed.NewI(100000000, 0))},
		}
	}

	b.ResetTimer()

	var totalOrders int64
	start := time.Now()

	for i := 0; i < b.N; i++ {
		userID := types.UserID(i%1000 + 1)
		result := e.Cmd(&PlaceOrderCmd{
			Req: &types.PlaceOrderRequest{
				UserID:   userID,
				Symbol:   "BTCUSDT",
				Category: constants.CATEGORY_SPOT,
				Side:     constants.ORDER_SIDE_BUY,
				Type:     constants.ORDER_TYPE_LIMIT,
				TIF:      constants.TIF_GTC,
				Quantity: types.Quantity(fixed.NewI(1, 0)),
				Price:    types.Price(fixed.NewI(50000, 0)),
			},
		})
		if result.Err == nil {
			atomic.AddInt64(&totalOrders, 1)
		}
	}

	duration := time.Since(start)
	b.ReportMetric(float64(totalOrders)/duration.Seconds(), "orders/sec")
	b.ReportMetric(float64(b.N)/duration.Seconds(), "attempted_ops/sec")

	runtime.GC()
	b.ReportMetric(float64(b.N)/duration.Seconds(), "ops/sec")

	e.Shutdown()
}
