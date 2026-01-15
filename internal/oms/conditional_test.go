package oms

import (
	"sync"
	"testing"

	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/robaho/fixed"
)

func TestConditionalIndex_Add(t *testing.T) {
	c := NewConditionalIndex()

	order := &types.Order{
		ID:           types.OrderID(1),
		UserID:       types.UserID(1),
		Symbol:       "BTCUSDT",
		Side:         constants.ORDER_SIDE_BUY,
		TriggerPrice: types.Price(fixed.NewI(49000, 0)),
	}

	c.Add(order)

	if c.buyTriggers["BTCUSDT"] == nil {
		t.Error("buyTriggers[BTCUSDT] should be created")
	}
}

func TestConditionalIndex_AddSell(t *testing.T) {
	c := NewConditionalIndex()

	order := &types.Order{
		ID:           types.OrderID(1),
		UserID:       types.UserID(1),
		Symbol:       "BTCUSDT",
		Side:         constants.ORDER_SIDE_SELL,
		TriggerPrice: types.Price(fixed.NewI(51000, 0)),
	}

	c.Add(order)

	if c.sellTriggers["BTCUSDT"] == nil {
		t.Error("sellTriggers[BTCUSDT] should be created")
	}
}

func TestConditionalIndex_Remove(t *testing.T) {
	c := NewConditionalIndex()

	order := &types.Order{
		ID:           types.OrderID(1),
		UserID:       types.UserID(1),
		Symbol:       "BTCUSDT",
		Side:         constants.ORDER_SIDE_BUY,
		TriggerPrice: types.Price(fixed.NewI(49000, 0)),
	}

	c.Add(order)
	c.Remove(order)

	if !c.deleted[&order.ID] {
		t.Error("order should be marked as deleted")
	}
}

func TestConditionalIndex_OnPriceTick_BuyTriggered(t *testing.T) {
	s := NewService()

	order := s.Create(types.UserID(1), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC,
		types.Price(fixed.NewI(50000, 0)), types.Quantity(fixed.NewI(10, 0)), types.Price(fixed.NewI(49000, 0)), false, false, constants.STOP_ORDER_TYPE_STOP)

	triggered := false
	s.OnPriceTick("BTCUSDT", types.Price(fixed.NewI(48500, 0)), func(o *types.Order) {
		triggered = true
		if o.ID != order.ID {
			t.Errorf("wrong order triggered")
		}
	})

	if !triggered {
		t.Error("order should be triggered")
	}
}

func TestConditionalIndex_OnPriceTick_BuyNotTriggered(t *testing.T) {
	s := NewService()

	s.Create(types.UserID(1), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC,
		types.Price(fixed.NewI(50000, 0)), types.Quantity(fixed.NewI(10, 0)), types.Price(fixed.NewI(49000, 0)), false, false, constants.STOP_ORDER_TYPE_STOP)

	triggered := false
	s.OnPriceTick("BTCUSDT", types.Price(fixed.NewI(49500, 0)), func(o *types.Order) {
		triggered = true
	})

	if triggered {
		t.Error("order should not be triggered")
	}
}

func TestConditionalIndex_OnPriceTick_SellTriggered(t *testing.T) {
	s := NewService()

	order := s.Create(types.UserID(1), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_SELL, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC,
		types.Price(fixed.NewI(50000, 0)), types.Quantity(fixed.NewI(10, 0)), types.Price(fixed.NewI(51000, 0)), false, false, constants.STOP_ORDER_TYPE_STOP)

	triggered := false
	s.OnPriceTick("BTCUSDT", types.Price(fixed.NewI(51500, 0)), func(o *types.Order) {
		triggered = true
		if o.ID != order.ID {
			t.Errorf("wrong order triggered")
		}
	})

	if !triggered {
		t.Error("order should be triggered")
	}
}

func TestConditionalIndex_OnPriceTick_Concurrent(t *testing.T) {
	s := NewService()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			s.Create(types.UserID(id), "BTCUSDT", constants.CATEGORY_LINEAR,
				constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC,
				types.Price(fixed.NewI(50000, 0)), types.Quantity(fixed.NewI(10, 0)), types.Price(fixed.NewI(int64(48000+id%10), 0)), false, false, constants.STOP_ORDER_TYPE_STOP)
		}(i)
	}

	wg.Wait()

	triggered := 0
	s.OnPriceTick("BTCUSDT", types.Price(fixed.NewI(47500, 0)), func(o *types.Order) {
		triggered++
	})

	if triggered != 100 {
		t.Errorf("expected 100 triggered orders, got %d", triggered)
	}
}

func BenchmarkConditionalIndex_Add(b *testing.B) {
	c := NewConditionalIndex()

	order := &types.Order{
		ID:           types.OrderID(0),
		UserID:       types.UserID(1),
		Symbol:       "BTCUSDT",
		Side:         constants.ORDER_SIDE_BUY,
		TriggerPrice: types.Price(fixed.NewI(49000, 0)),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		order.ID = types.OrderID(i)
		order.TriggerPrice = types.Price(fixed.NewI(int64(49000+i%100), 0))
		c.Add(order)
	}
}

func BenchmarkConditionalIndex_OnPriceTick(b *testing.B) {
	s := NewService()

	for i := 0; i < 1000; i++ {
		s.Create(types.UserID(i%10), "BTCUSDT", constants.CATEGORY_LINEAR,
			constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC,
			types.Price(fixed.NewI(50000, 0)), types.Quantity(fixed.NewI(10, 0)), types.Price(fixed.NewI(int64(49000+i%1000), 0)), false, false, constants.STOP_ORDER_TYPE_STOP)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.OnPriceTick("BTCUSDT", types.Price(fixed.NewI(49500, 0)), func(o *types.Order) {})
	}
}
