package engine

import (
	"flag"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/maxonlinux/meta-terminal-go/internal/balance"
	"github.com/maxonlinux/meta-terminal-go/internal/oms"
	"github.com/maxonlinux/meta-terminal-go/internal/registry"
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/robaho/fixed"
)

var (
	e2eUsers     = flag.Int("e2e-users", 1000, "E2E: number of concurrent users")
	e2eSymbols   = flag.Int("e2e-symbols", 10, "E2E: number of trading pairs")
	e2eOpsPerSec = flag.Int("e2e-ops", 10000, "E2E: target orders per second")
	e2eDuration  = flag.Duration("e2e-duration", 5*time.Second, "E2E: test duration")
)

type e2eStats struct {
	placedOrders int64
	filledOrders int64
	totalTrades  int64
	activeOrders int64
	errors       int64
	peakMemoryMB int64
	gcCycles     int32
	gcPauseNs    uint64
	startTime    time.Time
	endTime      time.Time
}

type e2eEngine struct {
	engine  *Engine
	store   *oms.Service
	users   []types.UserID
	symbols []string
	stats   *e2eStats
	stopCh  chan struct{}
}

func newE2EEngine() *e2eEngine {
	store := oms.NewService(nil)
	cb := &mockCallback{}
	e := NewEngine(store, nil, registry.New(), cb)

	symbols := []string{"BTCUSDT", "ETHUSDT", "BNBUSDT", "XRPUSDT", "SOLUSDT",
		"ADAUSDT", "DOGEUSDT", "DOTUSDT", "LINKUSDT", "MATICUSDT"}

	users := make([]types.UserID, *e2eUsers)
	for i := 0; i < *e2eUsers; i++ {
		users[i] = types.UserID(i + 1)
	}

	eng := &e2eEngine{
		engine:  e,
		store:   store,
		users:   users,
		symbols: symbols,
		stats:   &e2eStats{},
		stopCh:  make(chan struct{}),
	}

	for _, user := range users {
		for _, symbol := range symbols {
			eng.seedUserBalance(user, symbol)
		}
	}

	return eng
}

func (e *e2eEngine) seedUserBalance(userID types.UserID, symbol string) {
	base := balance.GetBaseAsset(symbol)
	quote := balance.GetQuoteAsset(symbol)
	amount := types.Quantity(fixed.NewI(1000000, 0)) // 1 million instead of 1 trillion

	if e.engine.portfolio.Balances[userID] == nil {
		e.engine.portfolio.Balances[userID] = make(map[string]*types.Balance)
	}

	e.engine.portfolio.Balances[userID][base] = &types.Balance{UserID: userID, Asset: base, Available: amount}
	e.engine.portfolio.Balances[userID][quote] = &types.Balance{UserID: userID, Asset: quote, Available: amount}
}

func (e *e2eEngine) shutdown() {
	close(e.stopCh)
	e.engine.Shutdown()
}

func (e *e2eEngine) generateOrder(userIdx int) *types.PlaceOrderRequest {
	user := e.users[userIdx%len(e.users)]
	symbol := e.symbols[userIdx%len(e.symbols)]

	var side int8 = constants.ORDER_SIDE_BUY
	if userIdx%2 == 0 {
		side = constants.ORDER_SIDE_SELL
	}

	var orderType int8 = constants.ORDER_TYPE_LIMIT
	if userIdx%5 == 0 {
		orderType = constants.ORDER_TYPE_MARKET
	}

	price := types.Price(fixed.NewI(int64(50000+userIdx%1000), 0))
	quantity := types.Quantity(fixed.NewI(int64(1+userIdx%10), 0))

	return &types.PlaceOrderRequest{
		UserID:   user,
		Symbol:   symbol,
		Category: constants.CATEGORY_LINEAR,
		Side:     side,
		Type:     orderType,
		TIF:      constants.TIF_GTC,
		Price:    price,
		Quantity: quantity,
	}
}

func (e *e2eEngine) runPlaceOrders(wg *sync.WaitGroup) {
	defer wg.Done()

	interval := time.Second / time.Duration(*e2eOpsPerSec/100)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	orderCount := 0
	for {
		select {
		case <-e.stopCh:
			return
		case <-ticker.C:
			if time.Since(e.stats.startTime) > *e2eDuration {
				return
			}

			req := e.generateOrder(orderCount)
			orderCount++

			result := e.engine.Cmd(&PlaceOrderCmd{Req: req})
			atomic.AddInt64(&e.stats.placedOrders, 1)

			if result.Err != nil {
				atomic.AddInt64(&e.stats.errors, 1)
				continue
			}

			order := result.Order
			switch order.Status {
			case constants.ORDER_STATUS_FILLED:
				atomic.AddInt64(&e.stats.filledOrders, 1)
				atomic.AddInt64(&e.stats.totalTrades, 1)
			case constants.ORDER_STATUS_NEW:
				atomic.AddInt64(&e.stats.activeOrders, 1)
			}
		}
	}
}

func (e *e2eEngine) runPriceTicks(wg *sync.WaitGroup) {
	defer wg.Done()

	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	symbolIdx := 0
	for {
		select {
		case <-e.stopCh:
			return
		case <-ticker.C:
			if time.Since(e.stats.startTime) > *e2eDuration {
				return
			}

			symbol := e.symbols[symbolIdx%len(e.symbols)]
			symbolIdx++

			price := types.Price(fixed.NewI(int64(50000+symbolIdx%100), 0))
			e.engine.OnPriceTick(symbol, price)
		}
	}
}

func (e *e2eEngine) collectMemoryStats(wg *sync.WaitGroup) {
	defer wg.Done()

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-e.stopCh:
			return
		case <-ticker.C:
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)
			currentMB := int64(memStats.Alloc / 1024 / 1024)
			peakMB := atomic.LoadInt64(&e.stats.peakMemoryMB)
			if currentMB > peakMB {
				atomic.StoreInt64(&e.stats.peakMemoryMB, currentMB)
			}
			e.stats.gcCycles = int32(memStats.NumGC)
			e.stats.gcPauseNs = memStats.PauseTotalNs
		}
	}
}

func (e *e2eEngine) printResults() {
	duration := e.stats.endTime.Sub(e.stats.startTime)
	ordersPerSecValue := float64(e.stats.placedOrders) / duration.Seconds()
	tradesPerSecValue := float64(e.stats.totalTrades) / duration.Seconds()

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("E2E BENCHMARK RESULTS - Order Placement Throughput")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Duration:              %v\n", duration.Round(time.Millisecond))
	fmt.Printf("Concurrent Users:      %d\n", *e2eUsers)
	fmt.Printf("Trading Pairs:         %d\n", *e2eSymbols)
	fmt.Printf("Target Rate:           %d ops/sec\n", *e2eOpsPerSec)
	fmt.Println(strings.Repeat("-", 60))
	fmt.Printf("Total Orders Placed:   %d\n", e.stats.placedOrders)
	fmt.Printf("Total Orders Filled:   %d\n", e.stats.filledOrders)
	fmt.Printf("Active Orders:         %d\n", e.stats.activeOrders)
	fmt.Printf("Total Errors:          %d\n", e.stats.errors)
	fmt.Println(strings.Repeat("-", 60))
	fmt.Printf("Throughput (orders):   %.0f ops/sec\n", ordersPerSecValue)
	fmt.Printf("Throughput (trades):   %.0f trades/sec\n", tradesPerSecValue)
	fmt.Printf("Peak Memory Usage:     %d MB\n", e.stats.peakMemoryMB)

	if e.stats.placedOrders > 0 {
		fillRate := float64(e.stats.filledOrders) * 100 / float64(e.stats.placedOrders)
		fmt.Printf("Fill Rate:             %.2f%%\n", fillRate)
	}

	fmt.Printf("GC Cycles:             %d\n", e.stats.gcCycles)
	fmt.Printf("Total GC Pause:        %v\n", time.Duration(e.stats.gcPauseNs))
	fmt.Println(strings.Repeat("=", 60))
}

func BenchmarkE2E_OrderPlacement(b *testing.B) {
	flag.Parse()

	b.ResetTimer()
	b.StopTimer()

	eng := newE2EEngine()
	eng.stats.startTime = time.Now()

	var wg sync.WaitGroup
	wg.Add(1)
	go eng.runPlaceOrders(&wg)

	wg.Add(1)
	go eng.runPriceTicks(&wg)

	wg.Add(1)
	go eng.collectMemoryStats(&wg)

	b.StartTimer()
	time.Sleep(*e2eDuration)
	b.StopTimer()

	eng.stats.endTime = time.Now()
	eng.shutdown()
	wg.Wait()

	eng.printResults()
}

func BenchmarkE2E_Scalability(b *testing.B) {
	flag.Parse()

	testCases := []struct {
		users    int
		symbols  int
		ops      int
		duration time.Duration
	}{
		{100, 5, 5000, 3 * time.Second},
		{500, 10, 10000, 5 * time.Second},
		{1000, 10, 15000, 5 * time.Second},
	}

	for _, tc := range testCases {
		b.Run(fmt.Sprintf("users=%d,symbols=%d,ops=%d", tc.users, tc.symbols, tc.ops), func(b *testing.B) {
			oldUsers := *e2eUsers
			oldSymbols := *e2eSymbols
			oldOps := *e2eOpsPerSec
			oldDuration := *e2eDuration

			*e2eUsers = tc.users
			*e2eSymbols = tc.symbols
			*e2eOpsPerSec = tc.ops
			*e2eDuration = tc.duration

			defer func() {
				*e2eUsers = oldUsers
				*e2eSymbols = oldSymbols
				*e2eOpsPerSec = oldOps
				*e2eDuration = oldDuration
			}()

			eng := newE2EEngine()
			eng.stats.startTime = time.Now()

			var wg sync.WaitGroup
			wg.Add(1)
			go eng.runPlaceOrders(&wg)
			wg.Add(1)
			go eng.runPriceTicks(&wg)
			wg.Add(1)
			go eng.collectMemoryStats(&wg)

			time.Sleep(*e2eDuration)
			eng.stats.endTime = time.Now()

			eng.shutdown()
			wg.Wait()

			fmt.Printf("Users: %d, Symbols: %d, Ops: %d\n", tc.users, tc.symbols, tc.ops)
			eng.printResults()
		})
	}
}
