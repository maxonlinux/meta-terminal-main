package oms

import (
	"context"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/orderbook"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func TestReduceOnlyCloseOnTrigger_ParallelChaos(t *testing.T) {
	s, portfolio := newTestService()
	portfolio.addPosition(1, "BTCUSDT", 10, constants.SIDE_LONG)

	ob := orderbook.New()
	s.orderbooks[constants.CATEGORY_LINEAR]["BTCUSDT"] = ob

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

	s.mu.RLock()
	defer s.mu.RUnlock()
	if userOrders, ok := s.orders[1]; ok {
		for _, order := range userOrders {
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
}
