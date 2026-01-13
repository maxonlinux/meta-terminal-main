package oms

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func TestActorOMS_RaceConditionFix_ConcurrentPositionAndMatching(t *testing.T) {
	oms, err := NewActorOMS(Config{}, &nopPortfolio{}, &nopClearing{})
	if err != nil {
		t.Fatalf("failed to create ActorOMS: %v", err)
	}

	const numGoroutines = 100
	const numIterations = 50

	var wg sync.WaitGroup
	var positionUpdates int64
	var ordersPlaced int64
	var errors int64

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < numIterations; j++ {
				userID := types.UserID((goroutineID % 10) + 1)

				select {
				case <-doPositionUpdate(oms, userID):
					atomic.AddInt64(&positionUpdates, 1)
				case err := <-doPlaceOrder(oms, userID):
					if err != nil {
						atomic.AddInt64(&errors, 1)
					} else {
						atomic.AddInt64(&ordersPlaced, 1)
					}
				}
			}
		}(i)
	}

	wg.Wait()

	t.Logf("Completed: %d position updates, %d orders placed, %d errors",
		positionUpdates, ordersPlaced, errors)

	if errors > 0 {
		t.Errorf("Had %d errors during concurrent execution", errors)
	}
}

func doPositionUpdate(oms *ActorOMS, userID types.UserID) <-chan struct{} {
	done := make(chan struct{}, 1)
	go func() {
		oms.OnPositionUpdate(userID, "BTCUSDT", int64(userID)*-1, constants.SIDE_SHORT)
		done <- struct{}{}
	}()
	return done
}

func doPlaceOrder(oms *ActorOMS, userID types.UserID) <-chan error {
	errChan := make(chan error, 1)
	go func() {
		result, err := oms.PlaceOrder(context.Background(), &types.OrderInput{
			UserID:     userID,
			Symbol:     "BTCUSDT",
			Category:   constants.CATEGORY_LINEAR,
			Side:       constants.ORDER_SIDE_SELL,
			Type:       constants.ORDER_TYPE_LIMIT,
			Quantity:   10,
			Price:      49000,
			ReduceOnly: true,
		})
		if err != nil {
			errChan <- err
		} else if result == nil {
			errChan <- nil
		} else {
			errChan <- nil
		}
	}()
	return errChan
}

func TestActorOMS_ConcurrentOrders_SameUser(t *testing.T) {
	oms, err := NewActorOMS(Config{}, &nopPortfolio{}, &nopClearing{})
	if err != nil {
		t.Fatalf("failed to create ActorOMS: %v", err)
	}

	userID := types.UserID(1)
	const numOrders = 100

	var wg sync.WaitGroup
	orderChan := make(chan *types.Order, numOrders)

	for i := 0; i < numOrders; i++ {
		wg.Add(1)
		go func(orderID int) {
			defer wg.Done()

			result, err := oms.PlaceOrder(context.Background(), &types.OrderInput{
				UserID:   userID,
				Symbol:   "BTCUSDT",
				Category: constants.CATEGORY_LINEAR,
				Side:     constants.ORDER_SIDE_BUY,
				Type:     constants.ORDER_TYPE_LIMIT,
				Quantity: types.Quantity(orderID + 1),
				Price:    types.Price(50000 - orderID),
			})
			if err == nil && result != nil && len(result.Orders) > 0 {
				orderChan <- result.Orders[0]
			}
		}(i)
	}

	wg.Wait()
	close(orderChan)

	orders := make([]*types.Order, 0, numOrders)
	for order := range orderChan {
		orders = append(orders, order)
	}

	if len(orders) != numOrders {
		t.Logf("Expected %d orders, got %d (some may have failed)", numOrders, len(orders))
	}

	t.Logf("Successfully placed %d orders concurrently for user %d", len(orders), userID)
}

func TestActorOMS_OrderCancellation(t *testing.T) {
	oms, err := NewActorOMS(Config{}, &nopPortfolio{}, &nopClearing{})
	if err != nil {
		t.Fatalf("failed to create ActorOMS: %v", err)
	}

	userID := types.UserID(1)

	result, err := oms.PlaceOrder(context.Background(), &types.OrderInput{
		UserID:   userID,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		Quantity: 100,
		Price:    50000,
	})
	if err != nil {
		t.Fatalf("PlaceOrder failed: %v", err)
	}

	if len(result.Orders) == 0 {
		t.Fatal("No order created")
	}

	orderID := result.Orders[0].ID

	err = oms.CancelOrder(userID, orderID)
	if err != nil {
		t.Errorf("CancelOrder failed: %v", err)
	}

	t.Logf("Order %d cancelled successfully", orderID)
}

func BenchmarkActorOMS_Throughput(b *testing.B) {
	oms, err := NewActorOMS(Config{}, &nopPortfolio{}, &nopClearing{})
	if err != nil {
		b.Fatalf("failed to create ActorOMS: %v", err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		userID := types.UserID(1)
		for pb.Next() {
			oms.PlaceOrder(context.Background(), &types.OrderInput{
				UserID:   userID,
				Symbol:   "BTCUSDT",
				Category: constants.CATEGORY_LINEAR,
				Side:     constants.ORDER_SIDE_BUY,
				Type:     constants.ORDER_TYPE_LIMIT,
				Quantity: 10,
				Price:    50000,
			})
		}
	})
}

func BenchmarkActorOMS_PositionUpdates(b *testing.B) {
	oms, err := NewActorOMS(Config{}, &nopPortfolio{}, &nopClearing{})
	if err != nil {
		b.Fatalf("failed to create ActorOMS: %v", err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			oms.OnPositionUpdate(1, "BTCUSDT", -5, constants.SIDE_SHORT)
		}
	})
}
