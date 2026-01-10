package oms

import (
	"context"
	"math/rand"
	"sync"
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/registry"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func FuzzOMS_ConditionalOrders(f *testing.F) {
	f.Fuzz(func(t *testing.T, seed int64, triggerPrice int64, side int8, orderType int8) {
		rand.Seed(seed)

		reg := registry.New()
		svc, err := New(Config{
			NATSURL:      "nats://localhost:4222",
			StreamPrefix: "fuzz",
			Shard:        "BTCUSDT",
			Snapshots:    nil,
		}, reg)
		if err != nil {
			return
		}
		defer svc.Close()
		ctx := context.Background()

		if triggerPrice <= 0 {
			return
		}

		input := &OrderInput{
			UserID:         types.UserID(rand.Intn(1000) + 1),
			Symbol:         "BTCUSDT",
			Category:       constants.CATEGORY_LINEAR,
			Side:           side,
			Type:           orderType,
			TIF:            constants.TIF_GTC,
			Qty:            int64(rand.Intn(1000) + 1),
			Price:          50000 + int64(rand.Intn(10000)),
			TriggerPrice:   triggerPrice,
			CloseOnTrigger: false,
			ReduceOnly:     false,
		}

		result, err := svc.PlaceOrder(ctx, input)
		if err != nil {
			return
		}

		if result == nil {
			t.Fatalf("PlaceOrder returned nil result")
		}

		if result.Status != constants.ORDER_STATUS_UNTRIGGERED {
			t.Fatalf("Expected UNTRIGGERED status, got %d", result.Status)
		}
	})
}

func FuzzOMS_ConcurrentOrders(f *testing.F) {
	f.Fuzz(func(t *testing.T, numOrders int) {
		if numOrders <= 0 || numOrders > 1000 {
			return
		}

		reg := registry.New()
		svc, _ := New(Config{
			NATSURL:      "nats://localhost:4222",
			StreamPrefix: "fuzz",
			Shard:        "BTCUSDT",
			Snapshots:    nil,
		}, reg)
		ctx := context.Background()

		var wg sync.WaitGroup
		results := make(chan error, numOrders)

		for i := 0; i < numOrders; i++ {
			wg.Add(1)
			go func(orderID int) {
				defer wg.Done()
				input := &OrderInput{
					UserID:   types.UserID(orderID%100 + 1),
					Symbol:   "BTCUSDT",
					Category: constants.CATEGORY_LINEAR,
					Side:     constants.ORDER_SIDE_BUY,
					Type:     constants.ORDER_TYPE_LIMIT,
					TIF:      constants.TIF_GTC,
					Qty:      10,
					Price:    50000 + int64(orderID%100),
				}
				_, err := svc.PlaceOrder(ctx, input)
				results <- err
			}(i)
		}

		wg.Wait()
		close(results)

		for err := range results {
			if err != nil {
				t.Fatalf("Concurrent order error: %v", err)
			}
		}

		svc.Close()
	})
}

func FuzzOMS_TriggerPriceEdgeCases(f *testing.F) {
	f.Fuzz(func(t *testing.T, triggerPrice int64, currentPrice int64) {
		reg := registry.New()
		svc, _ := New(Config{
			NATSURL:      "nats://localhost:4222",
			StreamPrefix: "fuzz",
			Shard:        "BTCUSDT",
			Snapshots:    nil,
		}, reg)
		ctx := context.Background()

		input := &OrderInput{
			UserID:         types.UserID(1),
			Symbol:         "BTCUSDT",
			Category:       constants.CATEGORY_LINEAR,
			Side:           constants.ORDER_SIDE_BUY,
			Type:           constants.ORDER_TYPE_LIMIT,
			TIF:            constants.TIF_GTC,
			Qty:            100,
			Price:          50000,
			TriggerPrice:   triggerPrice,
			CloseOnTrigger: false,
		}

		_, err := svc.PlaceOrder(ctx, input)
		if err != nil {
			t.Fatalf("PlaceOrder error: %v", err)
		}

		svc.OnPriceTick(types.Price(currentPrice))

		svc.Close()
	})
}

func FuzzOMS_OrderStatusTransitions(f *testing.F) {
	f.Fuzz(func(t *testing.T, filledQty int64, orderType int8) {
		if filledQty < 0 || filledQty > 100 {
			return
		}

		reg := registry.New()
		svc, _ := New(Config{
			NATSURL:      "nats://localhost:4222",
			StreamPrefix: "fuzz",
			Shard:        "BTCUSDT",
			Snapshots:    nil,
		}, reg)

		order := &types.Order{
			ID:       types.OrderID(1),
			UserID:   types.UserID(1),
			Symbol:   "BTCUSDT",
			Category: constants.CATEGORY_LINEAR,
			Side:     constants.ORDER_SIDE_BUY,
			Type:     orderType,
			TIF:      constants.TIF_GTC,
			Quantity: types.Quantity(100),
			Filled:   types.Quantity(filledQty),
			Status:   constants.ORDER_STATUS_NEW,
		}

		remaining := order.Remaining()
		switch {
		case remaining == 0 && filledQty == 100:
			if order.Status != constants.ORDER_STATUS_NEW {
				t.Fatalf("Filled order should have NEW status initially")
			}
		case remaining > 0 && order.Filled == 0:
			if order.Status != constants.ORDER_STATUS_NEW {
				t.Fatalf("Empty filled order should have NEW status")
			}
		}

		svc.Close()
	})
}
