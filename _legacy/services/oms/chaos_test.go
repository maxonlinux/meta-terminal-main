package oms

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/registry"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func TestOMS_ChaosConcurrentOrders(t *testing.T) {
	reg := registry.New()
	svc, _ := New(Config{
		NATSURL:      "nats://localhost:4222",
		StreamPrefix: "chaos",
		Shard:        "BTCUSDT",
		Snapshots:    nil,
	}, reg)
	defer svc.Close()
	ctx := context.Background()

	numGoroutines := 10
	numOrdersPerGoroutine := 10
	var wg sync.WaitGroup
	var successCount int32
	var errorCount int32

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < numOrdersPerGoroutine; j++ {
				input := &OrderInput{
					UserID:   types.UserID((goroutineID+j)%50 + 1),
					Symbol:   "BTCUSDT",
					Category: constants.CATEGORY_LINEAR,
					Side:     constants.ORDER_SIDE_BUY,
					Type:     constants.ORDER_TYPE_LIMIT,
					TIF:      constants.TIF_GTC,
					Qty:      10,
					Price:    50000 + int64((goroutineID+j)%1000),
				}

				_, err := svc.PlaceOrder(ctx, input)
				if err != nil {
					atomic.AddInt32(&errorCount, 1)
				} else {
					atomic.AddInt32(&successCount, 1)
				}

				time.Sleep(time.Duration(j%10) * time.Microsecond)
			}
		}(i)
	}

	wg.Wait()

	total := successCount + errorCount
	successRate := float64(successCount) / float64(total) * 100

	t.Logf("Chaos test results: success=%d, errors=%d, total=%d, success_rate=%.2f%%",
		successCount, errorCount, total, successRate)

	if successRate < 90 {
		t.Errorf("Success rate too low: %.2f%%", successRate)
	}
}

func TestOMS_ChaosRapidPlaceCancel(t *testing.T) {
	reg := registry.New()
	svc, _ := New(Config{
		NATSURL:      "nats://localhost:4222",
		StreamPrefix: "chaos",
		Shard:        "BTCUSDT",
		Snapshots:    nil,
	}, reg)
	defer svc.Close()
	ctx := context.Background()

	numIterations := 20
	var wg sync.WaitGroup
	var cancelErrors int32

	for i := 0; i < numIterations; i++ {
		wg.Add(1)
		go func(iterID int) {
			defer wg.Done()

			input := &OrderInput{
				UserID:   types.UserID(iterID%10 + 1),
				Symbol:   "BTCUSDT",
				Category: constants.CATEGORY_LINEAR,
				Side:     constants.ORDER_SIDE_BUY,
				Type:     constants.ORDER_TYPE_LIMIT,
				TIF:      constants.TIF_GTC,
				Qty:      5,
				Price:    50000 + int64(iterID%100),
			}

			result, _ := svc.PlaceOrder(ctx, input)
			if result != nil && result.OrderID != 0 {
				err := svc.CancelOrder(ctx, types.UserID(iterID%10+1), result.OrderID)
				if err != nil {
					atomic.AddInt32(&cancelErrors, 1)
				}
			}
		}(i)
	}

	wg.Wait()

	if cancelErrors > int32(numIterations/10) {
		t.Errorf("Too many cancel errors: %d/%d", cancelErrors, numIterations)
	}

	t.Logf("Rapid place/cancel test: errors=%d/%d", cancelErrors, numIterations)
}

func TestOMS_ChaosMixedOrderTypes(t *testing.T) {
	reg := registry.New()
	svc, _ := New(Config{
		NATSURL:      "nats://localhost:4222",
		StreamPrefix: "chaos",
		Shard:        "BTCUSDT",
		Snapshots:    nil,
	}, reg)
	defer svc.Close()
	ctx := context.Background()

	testCases := []struct {
		name  string
		input *OrderInput
	}{
		{
			name: "GTC_LIMIT_BUY",
			input: &OrderInput{
				UserID:   types.UserID(1),
				Symbol:   "BTCUSDT",
				Category: constants.CATEGORY_LINEAR,
				Side:     constants.ORDER_SIDE_BUY,
				Type:     constants.ORDER_TYPE_LIMIT,
				TIF:      constants.TIF_GTC,
				Qty:      100,
				Price:    49000,
			},
		},
		{
			name: "POST_ONLY_SELL",
			input: &OrderInput{
				UserID:   types.UserID(2),
				Symbol:   "BTCUSDT",
				Category: constants.CATEGORY_LINEAR,
				Side:     constants.ORDER_SIDE_SELL,
				Type:     constants.ORDER_TYPE_LIMIT,
				TIF:      constants.TIF_POST_ONLY,
				Qty:      100,
				Price:    51000,
			},
		},
		{
			name: "CONDITIONAL_BUY",
			input: &OrderInput{
				UserID:         types.UserID(3),
				Symbol:         "BTCUSDT",
				Category:       constants.CATEGORY_LINEAR,
				Side:           constants.ORDER_SIDE_BUY,
				Type:           constants.ORDER_TYPE_LIMIT,
				TIF:            constants.TIF_GTC,
				Qty:            50,
				Price:          48000,
				TriggerPrice:   47000,
				CloseOnTrigger: false,
			},
		},
	}

	var wg sync.WaitGroup
	for _, tc := range testCases {
		wg.Add(1)
		go func(name string, input *OrderInput) {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				input.UserID = types.UserID(i%5 + 1)
				result, err := svc.PlaceOrder(ctx, input)
				if err != nil {
					t.Errorf("%s: PlaceOrder error: %v", name, err)
				}
				if result != nil {
					svc.CancelOrder(ctx, input.UserID, result.OrderID)
				}
			}
		}(tc.name, tc.input)
	}

	wg.Wait()
	t.Logf("Mixed order types chaos test completed")
}

func TestOMS_ChaosHighFrequency(t *testing.T) {
	reg := registry.New()
	svc, _ := New(Config{
		NATSURL:      "nats://localhost:4222",
		StreamPrefix: "chaos",
		Shard:        "BTCUSDT",
		Snapshots:    nil,
	}, reg)
	defer svc.Close()
	ctx := context.Background()

	numOrders := 100
	start := time.Now()

	var wg sync.WaitGroup
	for i := 0; i < numOrders; i++ {
		wg.Add(1)
		go func(orderID int) {
			defer wg.Done()
			input := &OrderInput{
				UserID:   types.UserID(orderID % 100),
				Symbol:   "BTCUSDT",
				Category: constants.CATEGORY_LINEAR,
				Side:     constants.ORDER_SIDE_BUY,
				Type:     constants.ORDER_TYPE_LIMIT,
				TIF:      constants.TIF_GTC,
				Qty:      1,
				Price:    50000 + int64(orderID%100),
			}
			svc.PlaceOrder(ctx, input)
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)
	ordersPerSecond := float64(numOrders) / elapsed.Seconds()

	t.Logf("High frequency test: %d orders in %v (%.0f orders/sec)",
		numOrders, elapsed, ordersPerSecond)

	if ordersPerSecond < 5000 {
		t.Errorf("Throughput too low: %.0f orders/sec (target: >5000)", ordersPerSecond)
	}
}
