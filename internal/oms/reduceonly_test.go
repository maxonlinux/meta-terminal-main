package oms

import (
	"testing"

	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/robaho/fixed"
)

func TestReduceOnlyIndex_Add(t *testing.T) {
	r := NewReduceOnlyManager()

	order := &types.Order{
		ID:         types.OrderID(1),
		UserID:     types.UserID(1),
		Symbol:     "BTCUSDT",
		Side:       constants.ORDER_SIDE_BUY,
		Quantity:   types.Quantity(fixed.NewI(10, 0)),
		Filled:     types.Quantity(fixed.NewI(0, 0)),
		ReduceOnly: true,
	}

	r.Add(order)

	if r.exposure["BTCUSDT"][types.UserID(1)].Cmp(types.Quantity(fixed.NewI(10, 0))) != 0 {
		t.Errorf("expected exposure 10, got %d", r.exposure["BTCUSDT"][types.UserID(1)])
	}
}

func TestReduceOnlyIndex_AddNonReduceOnly(t *testing.T) {
	r := NewReduceOnlyManager()

	order := &types.Order{
		ID:         types.OrderID(1),
		UserID:     types.UserID(1),
		Symbol:     "BTCUSDT",
		Side:       constants.ORDER_SIDE_BUY,
		Quantity:   types.Quantity(fixed.NewI(10, 0)),
		Filled:     types.Quantity(fixed.NewI(0, 0)),
		ReduceOnly: false,
	}

	r.Add(order)

	if r.exposure["BTCUSDT"] != nil {
		t.Error("non-reduce-only order should not add exposure")
	}
}

func TestReduceOnlyIndex_Remove(t *testing.T) {
	r := NewReduceOnlyManager()

	order := &types.Order{
		ID:         types.OrderID(1),
		UserID:     types.UserID(1),
		Symbol:     "BTCUSDT",
		Side:       constants.ORDER_SIDE_BUY,
		Quantity:   types.Quantity(fixed.NewI(10, 0)),
		Filled:     types.Quantity(fixed.NewI(0, 0)),
		ReduceOnly: true,
	}

	r.Add(order)
	r.Remove(order)

	if r.exposure["BTCUSDT"][types.UserID(1)].Cmp(types.Quantity(fixed.NewI(0, 0))) != 0 {
		t.Errorf("expected exposure 0, got %d", r.exposure["BTCUSDT"][types.UserID(1)])
	}
}

func TestReduceOnlyIndex_OnPositionReduce(t *testing.T) {
	r := NewReduceOnlyManager()

	order1 := &types.Order{
		ID:         types.OrderID(1),
		UserID:     types.UserID(1),
		Symbol:     "BTCUSDT",
		Side:       constants.ORDER_SIDE_SELL,
		Quantity:   types.Quantity(fixed.NewI(10, 0)),
		Filled:     types.Quantity(fixed.NewI(0, 0)),
		ReduceOnly: true,
		Price:      types.Price(fixed.NewI(51000, 0)),
	}

	order2 := &types.Order{
		ID:         types.OrderID(2),
		UserID:     types.UserID(1),
		Symbol:     "BTCUSDT",
		Side:       constants.ORDER_SIDE_SELL,
		Quantity:   types.Quantity(fixed.NewI(10, 0)),
		Filled:     types.Quantity(fixed.NewI(0, 0)),
		ReduceOnly: true,
		Price:      types.Price(fixed.NewI(52000, 0)),
	}

	r.Add(order1)
	r.Add(order2)

	service := NewService()
	service.reduceonly = r
	service.OnPositionReduce(types.UserID(1), "BTCUSDT", types.Quantity(fixed.NewI(5, 0)))

	if order1.Status != constants.ORDER_STATUS_CANCELED {
		t.Errorf("order1 should be fully canceled, got status %d", order1.Status)
	}
}

func BenchmarkReduceOnlyIndex_Add(b *testing.B) {
	r := NewReduceOnlyManager()

	order := &types.Order{
		ID:         types.OrderID(0),
		UserID:     types.UserID(1),
		Symbol:     "BTCUSDT",
		Side:       constants.ORDER_SIDE_BUY,
		Quantity:   types.Quantity(fixed.NewI(10, 0)),
		Filled:     types.Quantity(fixed.NewI(0, 0)),
		ReduceOnly: true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		order.ID = types.OrderID(i)
		r.Add(order)
	}
}

func BenchmarkReduceOnlyIndex_OnPositionReduce(b *testing.B) {
	r := NewReduceOnlyManager()

	for i := 0; i < 1000; i++ {
		order := &types.Order{
			ID:         types.OrderID(i),
			UserID:     types.UserID(1),
			Symbol:     "BTCUSDT",
			Side:       constants.ORDER_SIDE_SELL,
			Quantity:   types.Quantity(fixed.NewI(10, 0)),
			Filled:     types.Quantity(fixed.NewI(0, 0)),
			ReduceOnly: true,
			Price:      types.Price(fixed.NewI(int64(50000+i), 0)),
		}
		r.Add(order)
	}

	service := NewService()
	service.reduceonly = r

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.OnPositionReduce(types.UserID(1), "BTCUSDT", types.Quantity(fixed.NewI(5000, 0)))
	}
}
