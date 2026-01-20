package engine

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/maxonlinux/meta-terminal-go/internal/oms"
	"github.com/maxonlinux/meta-terminal-go/internal/registry"
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/persistence"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/robaho/fixed"
)

func BenchmarkEngine_Throughput_NoPersistence(b *testing.B) {
	reg := registry.New()
	for _, symbol := range []string{"BTCUSDT", "ETHUSDT", "BNBUSDT", "SOLUSDT", "XRPUSDT"} {
		reg.SetInstrument(symbol, registry.FromSymbol(symbol, 50000))
	}

	store := oms.NewService(nil)
	cb := &mockCallback{}
	e := NewEngine(store, reg, cb)

	numUsers := 1000
	users := make([]types.UserID, numUsers)
	for i := range users {
		users[i] = types.UserID(i + 1)
	}

	for _, user := range users {
		e.portfolio.Balances[user] = map[string]*types.Balance{
			"USDT": {UserID: user, Asset: "USDT", Available: types.Quantity(fixed.NewI(100000000, 0))},
			"BTC":  {UserID: user, Asset: "BTC", Available: types.Quantity(fixed.NewI(1000, 0))},
			"ETH":  {UserID: user, Asset: "ETH", Available: types.Quantity(fixed.NewI(10000, 0))},
		}
	}

	b.ResetTimer()
	b.SetBytes(1)

	var totalOrders int64
	var wg sync.WaitGroup
	start := time.Now()

	for i := 0; i < b.N; i++ {
		user := users[i%numUsers]
		symbol := []string{"BTCUSDT", "ETHUSDT", "BNBUSDT", "SOLUSDT", "XRPUSDT"}[i%5]

		result := e.Cmd(&PlaceOrderCmd{
			Req: &types.PlaceOrderRequest{
				UserID:   user,
				Symbol:   symbol,
				Category: constants.CATEGORY_SPOT,
				Side:     constants.ORDER_SIDE_BUY,
				Type:     constants.ORDER_TYPE_LIMIT,
				TIF:      constants.TIF_GTC,
				Quantity: types.Quantity(fixed.NewI(1, 0)),
				Price:    types.Price(fixed.NewI(50000, 0)),
			},
		})

		if result.Err == nil && result.Order != nil {
			atomic.AddInt64(&totalOrders, 1)
		}

		if (i+1)%10000 == 0 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				e.OnPriceTick("BTCUSDT", types.Price(fixed.NewI(50001, 0)))
			}()
		}
	}

	wg.Wait()
	duration := time.Since(start)
	ordersPerSec := float64(totalOrders) / duration.Seconds()

	b.ReportMetric(ordersPerSec, "orders/sec")
	b.ReportMetric(float64(totalOrders)/float64(b.N)*100, "success_rate_%")
	b.ReportMetric(duration.Seconds(), "total_time_s")
	b.ReportMetric(float64(b.N)/duration.Seconds(), "attempted_ops/sec")

	e.Shutdown()
}

func BenchmarkEngine_Throughput_WithPersistence(b *testing.B) {
	reg := registry.New()
	for _, symbol := range []string{"BTCUSDT", "ETHUSDT", "BNBUSDT", "SOLUSDT", "XRPUSDT"} {
		reg.SetInstrument(symbol, registry.FromSymbol(symbol, 50000))
	}

	pkv, err := persistence.Open(b.TempDir())
	if err != nil {
		b.Fatalf("open pebblekv: %v", err)
	}
	defer pkv.Close()

	store := oms.NewService(pkv)
	cb := &mockCallback{}
	e := NewEngine(store, reg, cb)

	numUsers := 1000
	users := make([]types.UserID, numUsers)
	for i := range users {
		users[i] = types.UserID(i + 1)
	}

	for _, user := range users {
		e.portfolio.Balances[user] = map[string]*types.Balance{
			"USDT": {UserID: user, Asset: "USDT", Available: types.Quantity(fixed.NewI(100000000, 0))},
			"BTC":  {UserID: user, Asset: "BTC", Available: types.Quantity(fixed.NewI(1000, 0))},
			"ETH":  {UserID: user, Asset: "ETH", Available: types.Quantity(fixed.NewI(10000, 0))},
		}
	}

	b.ResetTimer()
	b.SetBytes(1)

	var totalOrders int64
	var wg sync.WaitGroup
	start := time.Now()

	for i := 0; i < b.N; i++ {
		user := users[i%numUsers]
		symbol := []string{"BTCUSDT", "ETHUSDT", "BNBUSDT", "SOLUSDT", "XRPUSDT"}[i%5]

		result := e.Cmd(&PlaceOrderCmd{
			Req: &types.PlaceOrderRequest{
				UserID:   user,
				Symbol:   symbol,
				Category: constants.CATEGORY_SPOT,
				Side:     constants.ORDER_SIDE_BUY,
				Type:     constants.ORDER_TYPE_LIMIT,
				TIF:      constants.TIF_GTC,
				Quantity: types.Quantity(fixed.NewI(1, 0)),
				Price:    types.Price(fixed.NewI(50000, 0)),
			},
		})

		if result.Err == nil && result.Order != nil {
			atomic.AddInt64(&totalOrders, 1)
		}

		if (i+1)%10000 == 0 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				e.OnPriceTick("BTCUSDT", types.Price(fixed.NewI(50001, 0)))
			}()
		}
	}

	wg.Wait()
	duration := time.Since(start)
	ordersPerSec := float64(totalOrders) / duration.Seconds()

	b.ReportMetric(ordersPerSec, "orders/sec")
	b.ReportMetric(float64(totalOrders)/float64(b.N)*100, "success_rate_%")
	b.ReportMetric(duration.Seconds(), "total_time_s")
	b.ReportMetric(float64(b.N)/duration.Seconds(), "attempted_ops/sec")

	e.Shutdown()
}

func BenchmarkEngine_ConcurrentOrders_NoPersistence(b *testing.B) {
	reg := registry.New()
	reg.SetInstrument("BTCUSDT", registry.FromSymbol("BTCUSDT", 50000))

	store := oms.NewService(nil)
	cb := &mockCallback{}
	e := NewEngine(store, reg, cb)

	numGoroutines := 100
	ordersPerGoroutine := b.N / numGoroutines

	for i := 0; i < 1000; i++ {
		userID := types.UserID(i + 1)
		e.portfolio.Balances[userID] = map[string]*types.Balance{
			"USDT": {UserID: userID, Asset: "USDT", Available: types.Quantity(fixed.NewI(100000000, 0))},
		}
	}

	b.ResetTimer()

	var totalOrders int64
	var wg sync.WaitGroup
	start := time.Now()

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			localOrders := int64(0)
			for i := 0; i < ordersPerGoroutine; i++ {
				userID := types.UserID((goroutineID*ordersPerGoroutine+i)%1000 + 1)
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
					localOrders++
				}
			}
			atomic.AddInt64(&totalOrders, localOrders)
		}(g)
	}

	wg.Wait()
	duration := time.Since(start)
	ordersPerSec := float64(totalOrders) / duration.Seconds()

	b.ReportMetric(ordersPerSec, "orders/sec")
	b.ReportMetric(float64(totalOrders)/float64(b.N)*100, "success_rate_%")
	b.ReportMetric(float64(b.N)/duration.Seconds(), "attempted_ops/sec")

	e.Shutdown()
}

func BenchmarkEngine_MatchingOnly(b *testing.B) {
	reg := registry.New()
	reg.SetInstrument("BTCUSDT", registry.FromSymbol("BTCUSDT", 50000))

	store := oms.NewService(nil)
	cb := &mockCallback{}
	e := NewEngine(store, reg, cb)

	for i := 0; i < 1000; i++ {
		userID := types.UserID(i + 1)
		e.portfolio.Balances[userID] = map[string]*types.Balance{
			"USDT": {UserID: userID, Asset: "USDT", Available: types.Quantity(fixed.NewI(100000000, 0))},
			"BTC":  {UserID: userID, Asset: "BTC", Available: types.Quantity(fixed.NewI(1000, 0))},
		}
	}

	for i := 0; i < 5000; i++ {
		e.Cmd(&PlaceOrderCmd{
			Req: &types.PlaceOrderRequest{
				UserID:   types.UserID(i%1000 + 1),
				Symbol:   "BTCUSDT",
				Category: constants.CATEGORY_SPOT,
				Side:     constants.ORDER_SIDE_SELL,
				Type:     constants.ORDER_TYPE_LIMIT,
				TIF:      constants.TIF_GTC,
				Quantity: types.Quantity(fixed.NewI(1, 0)),
				Price:    types.Price(fixed.NewI(50000+int64(i%100), 0)),
			},
		})
	}

	b.ResetTimer()
	b.SetBytes(1)

	var totalFilled int64
	start := time.Now()

	for i := 0; i < b.N; i++ {
		result := e.Cmd(&PlaceOrderCmd{
			Req: &types.PlaceOrderRequest{
				UserID:   types.UserID(i%1000 + 1),
				Symbol:   "BTCUSDT",
				Category: constants.CATEGORY_SPOT,
				Side:     constants.ORDER_SIDE_BUY,
				Type:     constants.ORDER_TYPE_LIMIT,
				TIF:      constants.TIF_GTC,
				Quantity: types.Quantity(fixed.NewI(1, 0)),
				Price:    types.Price(fixed.NewI(50050, 0)),
			},
		})

		if result.Err == nil && result.Order != nil && result.Order.Status == constants.ORDER_STATUS_FILLED {
			totalFilled++
		}
	}

	duration := time.Since(start)
	tradesPerSec := float64(totalFilled) / duration.Seconds()

	b.ReportMetric(tradesPerSec, "trades/sec")
	b.ReportMetric(float64(totalFilled)/float64(b.N)*100, "fill_rate_%")
	b.ReportMetric(duration.Seconds(), "total_time_s")

	e.Shutdown()
}
