package oms

import (
	"context"
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/messaging"
	"github.com/anomalyco/meta-terminal-go/internal/orderbook"
	"github.com/anomalyco/meta-terminal-go/internal/pool"
	"github.com/anomalyco/meta-terminal-go/internal/registry"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func BenchmarkOMS_PlaceOrder(b *testing.B) {
	cfg := OrderInput{
		UserID:   types.UserID(0),
		Symbol:   "BTCUSDT",
		Category: 1,
		Side:     1,
		Type:     0,
		TIF:      0,
		Qty:      100,
		Price:    50000,
	}

	reg := registry.New()
	svc, _ := New(Config{
		NATSURL:      "nats://localhost:4222",
		StreamPrefix: "bench",
		Shard:        "BTCUSDT",
		Snapshots:    nil,
	}, reg)
	ctx := context.Background()

	for i := 0; i < 1000; i++ {
		ob := orderbook.New()
		ob.AddResting(&types.Order{
			ID:        types.OrderID(i + 1),
			UserID:    types.UserID(i + 1),
			Symbol:    "BTCUSDT",
			Category:  1,
			Side:      1,
			Type:      0,
			TIF:       0,
			Quantity:  types.Quantity(100),
			Price:     types.Price(50000 - i%100),
			Status:    0,
			CreatedAt: types.NowNano(),
		})
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		input := cfg
		input.UserID = types.UserID(i + 10000)
		svc.PlaceOrder(ctx, &input)
	}
}

func BenchmarkOMS_MatchOrder(b *testing.B) {
	reg := registry.New()
	svc, _ := New(Config{
		NATSURL:      "nats://localhost:4222",
		StreamPrefix: "bench",
		Shard:        "BTCUSDT",
		Snapshots:    nil,
	}, reg)
	ctx := context.Background()

	for i := 0; i < 1000; i++ {
		input := OrderInput{
			UserID:   types.UserID(i + 10000),
			Symbol:   "BTCUSDT",
			Category: 1,
			Side:     0,
			Type:     0,
			TIF:      0,
			Qty:      100,
			Price:    50000,
		}
		svc.PlaceOrder(ctx, &input)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		input := OrderInput{
			UserID:   types.UserID(1),
			Symbol:   "BTCUSDT",
			Category: 1,
			Side:     0,
			Type:     0,
			TIF:      0,
			Qty:      100,
			Price:    50500,
		}
		svc.PlaceOrder(ctx, &input)
	}
}

func BenchmarkNATS_Publish(b *testing.B) {
	n, _ := messaging.New(messaging.Config{
		URL:          "nats://localhost:4222",
		StreamPrefix: "bench",
	})
	defer n.Close()
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	// Publish Gob-encoded OrderInput as an example
	input := types.OrderInput{
		UserID:   types.UserID(1),
		Symbol:   "BTCUSDT",
		Category: 1,
		Side:     0,
		Type:     0,
		TIF:      0,
		Quantity: types.Quantity(100),
		Price:    types.Price(50000),
	}
	for i := 0; i < b.N; i++ {
		_ = n.PublishGob(ctx, "test.subject", &input)
	}
}

func BenchmarkPool_GetOrder(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		order := pool.GetOrder()
		pool.PutOrder(order)
	}
}

func BenchmarkPool_GetTrade(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		trade := pool.GetTrade()
		pool.PutTrade(trade)
	}
}

func BenchmarkOMS_PlaceConditionalOrder(b *testing.B) {
	reg := registry.New()
	svc, _ := New(Config{
		NATSURL:      "nats://localhost:4222",
		StreamPrefix: "bench",
		Shard:        "BTCUSDT",
		Snapshots:    nil,
	}, reg)
	ctx := context.Background()

	input := OrderInput{
		UserID:         types.UserID(1),
		Symbol:         "BTCUSDT",
		Category:       1,
		Side:           0,
		Type:           0,
		TIF:            0,
		Qty:            100,
		Price:          50000,
		TriggerPrice:   51000,
		CloseOnTrigger: false,
		ReduceOnly:     false,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		svc.PlaceOrder(ctx, &input)
	}
}

func BenchmarkOMS_OnPriceTick(b *testing.B) {
	reg := registry.New()
	svc, _ := New(Config{
		NATSURL:      "nats://localhost:4222",
		StreamPrefix: "bench",
		Shard:        "BTCUSDT",
		Snapshots:    nil,
	}, reg)
	ctx := context.Background()

	input := OrderInput{
		UserID:         types.UserID(1),
		Symbol:         "BTCUSDT",
		Category:       1,
		Side:           0,
		Type:           0,
		TIF:            0,
		Qty:            100,
		Price:          50000,
		TriggerPrice:   51000,
		CloseOnTrigger: false,
		ReduceOnly:     false,
	}

	for i := 0; i < 1000; i++ {
		input.UserID = types.UserID(i + 1)
		svc.PlaceOrder(ctx, &input)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		svc.OnPriceTick(types.Price(51100))
	}
}

func BenchmarkOMS_HandleCloseOnTrigger(b *testing.B) {
	reg := registry.New()
	svc, _ := New(Config{
		NATSURL:      "nats://localhost:4222",
		StreamPrefix: "bench",
		Shard:        "BTCUSDT",
		Snapshots:    nil,
	}, reg)

	order := pool.GetOrder()
	order.ID = types.OrderID(1)
	order.UserID = types.UserID(1)
	order.Symbol = "BTCUSDT"
	order.Category = 1
	order.Side = 1
	order.Type = 0
	order.TIF = 0
	order.Quantity = types.Quantity(100)
	order.Price = types.Price(50000)
	order.TriggerPrice = types.Price(51000)
	order.CloseOnTrigger = true
	order.ReduceOnly = false

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		svc.handleCloseOnTrigger(order)
	}
}

func BenchmarkOMS_AdjustReduceOnlyOrders(b *testing.B) {
	reg := registry.New()
	svc, _ := New(Config{
		NATSURL:      "nats://localhost:4222",
		StreamPrefix: "bench",
		Shard:        "BTCUSDT",
		Snapshots:    nil,
	}, reg)

	svc.mu.Lock()
	if svc.orders[types.UserID(1)] == nil {
		svc.orders[types.UserID(1)] = make(map[types.OrderID]*types.Order)
	}
	for i := 0; i < 100; i++ {
		order := pool.GetOrder()
		order.ID = types.OrderID(i + 1)
		order.UserID = types.UserID(1)
		order.Symbol = "BTCUSDT"
		order.Category = 1
		order.Side = 1
		order.Type = 0
		order.TIF = 0
		order.Quantity = types.Quantity(100)
		order.Price = types.Price(50000)
		order.ReduceOnly = true
		order.Status = constants.ORDER_STATUS_NEW
		svc.orders[types.UserID(1)][order.ID] = order
	}
	svc.mu.Unlock()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		svc.adjustReduceOnlyOrders(types.UserID(1), "BTCUSDT", 50)
	}
}
