package oms

import (
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/orderbook"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func TestReduceOnly_ReactiveClampOnPositionShrink(t *testing.T) {
	s, portfolio := newTestService()
	portfolio.addPosition(1, "BTCUSDT", 5, constants.SIDE_LONG)

	ob := orderbook.New()
	s.orderbooks[constants.CATEGORY_LINEAR]["BTCUSDT"] = ob

	order1 := &types.Order{
		ID:         1,
		UserID:     1,
		Symbol:     "BTCUSDT",
		Category:   constants.CATEGORY_LINEAR,
		Side:       constants.ORDER_SIDE_SELL,
		Type:       constants.ORDER_TYPE_LIMIT,
		TIF:        constants.TIF_GTC,
		Status:     constants.ORDER_STATUS_NEW,
		Price:      100,
		Quantity:   3,
		ReduceOnly: true,
	}
	order2 := &types.Order{
		ID:         2,
		UserID:     1,
		Symbol:     "BTCUSDT",
		Category:   constants.CATEGORY_LINEAR,
		Side:       constants.ORDER_SIDE_SELL,
		Type:       constants.ORDER_TYPE_LIMIT,
		TIF:        constants.TIF_GTC,
		Status:     constants.ORDER_STATUS_NEW,
		Price:      101,
		Quantity:   3,
		ReduceOnly: true,
	}
	s.storeOrder(order1)
	s.storeOrder(order2)
	ob.Add(order1)
	ob.Add(order2)

	s.updateReduceOnlyCommitment(order1, 3)
	s.updateReduceOnlyCommitment(order2, 3)

	s.OnPositionUpdate(1, "BTCUSDT", 2, constants.SIDE_LONG)

	if order1.Quantity != 2 {
		t.Fatalf("expected order1 clamped to 2, got %d", order1.Quantity)
	}
	if order2.Quantity != 0 {
		t.Fatalf("expected order2 clamped to 0, got %d", order2.Quantity)
	}
	if s.reduceOnlyCommitment[1]["BTCUSDT"] != 2 {
		t.Fatalf("expected commitment 2, got %d", s.reduceOnlyCommitment[1]["BTCUSDT"])
	}
}
