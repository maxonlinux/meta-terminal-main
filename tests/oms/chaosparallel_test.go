package oms_test

import (
	"context"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func TestReduceOnlyCloseOnTriggerParallelChaos(t *testing.T) {
	s, port := newService()
	setBalance(port, 1, "USDT", 1_000_000_000, 0, 0)
	setPosition(port, 1, "BTCUSDT", 10, constants.SIDE_LONG, 50000, 10)

	for i := 0; i < 5; i++ {
		input := &types.OrderInput{
			UserID:     1,
			Symbol:     "BTCUSDT",
			Category:   constants.CATEGORY_LINEAR,
			Side:       constants.ORDER_SIDE_SELL,
			Type:       constants.ORDER_TYPE_LIMIT,
			TIF:        constants.TIF_GTC,
			Quantity:   2,
			Price:      100 + types.Price(i),
			ReduceOnly: true,
		}
		if _, err := s.PlaceOrder(context.Background(), input); err != nil {
			t.Fatalf("reduceOnly place error: %v", err)
		}
	}

	for i := 0; i < 3; i++ {
		input := &types.OrderInput{
			UserID:         1,
			Symbol:         "BTCUSDT",
			Category:       constants.CATEGORY_LINEAR,
			Side:           constants.ORDER_SIDE_SELL,
			Type:           constants.ORDER_TYPE_LIMIT,
			TIF:            constants.TIF_GTC,
			Quantity:       3,
			Price:          90,
			TriggerPrice:   80,
			CloseOnTrigger: true,
		}
		if _, err := s.PlaceOrder(context.Background(), input); err != nil {
			t.Fatalf("closeOnTrigger place error: %v", err)
		}
	}

	var mu sync.Mutex
	lastSize := int64(10)
	wg := sync.WaitGroup{}
	wg.Add(8)

	for i := 0; i < 8; i++ {
		go func(seed int64) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(time.Now().UnixNano() + seed))
			for j := 0; j < 200; j++ {
				size := int64(rng.Intn(11))
				mu.Lock()
				lastSize = size
				mu.Unlock()
				s.OnPositionUpdate(1, "BTCUSDT", size, constants.SIDE_LONG)
			}
		}(int64(i))
	}

	wg.Wait()

	mu.Lock()
	finalSize := lastSize
	mu.Unlock()
	maxAllowed := absInt64(finalSize)

	orders := s.GetOrders(1)
	for _, order := range orders {
		if order.Symbol != "BTCUSDT" {
			continue
		}
		if order.Status != constants.ORDER_STATUS_NEW && order.Status != constants.ORDER_STATUS_UNTRIGGERED {
			continue
		}
		if order.ReduceOnly || order.CloseOnTrigger {
			if int64(order.Quantity) > maxAllowed {
				t.Fatalf("order %d exceeds position size: qty=%d allowed=%d", order.ID, order.Quantity, maxAllowed)
			}
		}
	}
}

func absInt64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
