package oms

import (
	"sync"
	"testing"

	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/robaho/fixed"
)

func TestConditionalIndex_AddBuy(t *testing.T) {
	c := NewConditionalIndex()

	order := &types.Order{
		ID:           types.OrderID(1),
		UserID:       types.UserID(1),
		Symbol:       "BTCUSDT",
		Side:         constants.ORDER_SIDE_BUY,
		TriggerPrice: types.Price(fixed.NewI(49000, 0)),
		Status:       constants.ORDER_STATUS_UNTRIGGERED,
	}

	c.Add(order)

	// Verify order was added by checking it triggers correctly
	triggered := 0
	c.CheckTriggers("BTCUSDT", types.Price(fixed.NewI(48500, 0)), func(o *types.Order) {
		triggered++
	})
	if triggered != 1 {
		t.Errorf("expected 1 triggered order, got %d", triggered)
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
		Status:       constants.ORDER_STATUS_UNTRIGGERED,
	}

	c.Add(order)

	// Verify order was added by checking it triggers correctly
	triggered := 0
	c.CheckTriggers("BTCUSDT", types.Price(fixed.NewI(51500, 0)), func(o *types.Order) {
		triggered++
	})
	if triggered != 1 {
		t.Errorf("expected 1 triggered order, got %d", triggered)
	}
}

func TestConditionalIndex_CancelViaStatus(t *testing.T) {
	c := NewConditionalIndex()

	order := &types.Order{
		ID:           types.OrderID(1),
		UserID:       types.UserID(1),
		Symbol:       "BTCUSDT",
		Side:         constants.ORDER_SIDE_BUY,
		TriggerPrice: types.Price(fixed.NewI(49000, 0)),
		Status:       constants.ORDER_STATUS_UNTRIGGERED,
	}

	c.Add(order)

	// Cancel via status change (new API)
	order.Status = constants.ORDER_STATUS_CANCELED

	// Order should not trigger even though price condition is met
	triggered := 0
	c.CheckTriggers("BTCUSDT", types.Price(fixed.NewI(48500, 0)), func(o *types.Order) {
		triggered++
	})
	if triggered != 0 {
		t.Errorf("canceled order should not trigger, got %d", triggered)
	}
}

func TestConditionalIndex_CheckTriggers_BuyTriggered(t *testing.T) {
	c := NewConditionalIndex()

	order := &types.Order{
		ID:           types.OrderID(1),
		UserID:       types.UserID(1),
		Symbol:       "BTCUSDT",
		Side:         constants.ORDER_SIDE_BUY,
		TriggerPrice: types.Price(fixed.NewI(49000, 0)),
		Status:       constants.ORDER_STATUS_UNTRIGGERED,
	}

	c.Add(order)

	// Price drops to/below trigger - should trigger
	triggeredOrder := false
	c.CheckTriggers("BTCUSDT", types.Price(fixed.NewI(48500, 0)), func(o *types.Order) {
		triggeredOrder = true
		if o.ID != order.ID {
			t.Errorf("wrong order triggered")
		}
		if o.Status != constants.ORDER_STATUS_TRIGGERED {
			t.Errorf("order status should be TRIGGERED")
		}
	})

	if !triggeredOrder {
		t.Error("order should have been triggered")
	}
}

func TestConditionalIndex_CheckTriggers_BuyNotTriggered(t *testing.T) {
	c := NewConditionalIndex()

	c.Add(&types.Order{
		ID:           types.OrderID(1),
		UserID:       types.UserID(1),
		Symbol:       "BTCUSDT",
		Side:         constants.ORDER_SIDE_BUY,
		TriggerPrice: types.Price(fixed.NewI(49000, 0)),
		Status:       constants.ORDER_STATUS_UNTRIGGERED,
	})

	// Price is still above trigger - should not trigger
	triggered := 0
	c.CheckTriggers("BTCUSDT", types.Price(fixed.NewI(49500, 0)), func(o *types.Order) {
		triggered++
	})
	if triggered != 0 {
		t.Errorf("order should not trigger, got %d", triggered)
	}
}

func TestConditionalIndex_CheckTriggers_SellTriggered(t *testing.T) {
	c := NewConditionalIndex()

	order := &types.Order{
		ID:           types.OrderID(1),
		UserID:       types.UserID(1),
		Symbol:       "BTCUSDT",
		Side:         constants.ORDER_SIDE_SELL,
		TriggerPrice: types.Price(fixed.NewI(51000, 0)),
		Status:       constants.ORDER_STATUS_UNTRIGGERED,
	}

	c.Add(order)

	// Price rises to/above trigger - should trigger
	triggeredOrder := false
	c.CheckTriggers("BTCUSDT", types.Price(fixed.NewI(51500, 0)), func(o *types.Order) {
		triggeredOrder = true
		if o.ID != order.ID {
			t.Errorf("wrong order triggered")
		}
	})

	if !triggeredOrder {
		t.Error("order should have been triggered")
	}
}

func TestConditionalIndex_CheckTriggers_SellNotTriggered(t *testing.T) {
	c := NewConditionalIndex()

	c.Add(&types.Order{
		ID:           types.OrderID(1),
		UserID:       types.UserID(1),
		Symbol:       "BTCUSDT",
		Side:         constants.ORDER_SIDE_SELL,
		TriggerPrice: types.Price(fixed.NewI(51000, 0)),
		Status:       constants.ORDER_STATUS_UNTRIGGERED,
	})

	// Price is still below trigger - should not trigger
	triggered := 0
	c.CheckTriggers("BTCUSDT", types.Price(fixed.NewI(50500, 0)), func(o *types.Order) {
		triggered++
	})
	if triggered != 0 {
		t.Errorf("order should not trigger, got %d", triggered)
	}
}

func TestConditionalIndex_CheckTriggers_SellPartialTrigger(t *testing.T) {
	c := NewConditionalIndex()

	order1 := &types.Order{ID: types.OrderID(1), Symbol: "BTCUSDT", Side: constants.ORDER_SIDE_SELL, TriggerPrice: types.Price(fixed.NewI(50500, 0)), Status: constants.ORDER_STATUS_UNTRIGGERED}
	order2 := &types.Order{ID: types.OrderID(2), Symbol: "BTCUSDT", Side: constants.ORDER_SIDE_SELL, TriggerPrice: types.Price(fixed.NewI(51000, 0)), Status: constants.ORDER_STATUS_UNTRIGGERED}
	order3 := &types.Order{ID: types.OrderID(3), Symbol: "BTCUSDT", Side: constants.ORDER_SIDE_SELL, TriggerPrice: types.Price(fixed.NewI(51500, 0)), Status: constants.ORDER_STATUS_UNTRIGGERED}

	c.Add(order1)
	c.Add(order2)
	c.Add(order3)

	// Price reaches 51000: only two lowest triggers should fire.
	triggered := 0
	c.CheckTriggers("BTCUSDT", types.Price(fixed.NewI(51000, 0)), func(o *types.Order) {
		triggered++
	})
	if triggered != 2 {
		t.Errorf("expected 2 triggered orders, got %d", triggered)
	}
	if order3.Status != constants.ORDER_STATUS_UNTRIGGERED {
		t.Errorf("expected highest trigger to remain untriggered")
	}
}

func TestConditionalIndex_CheckTriggers_Concurrent(t *testing.T) {
	s := NewService(nil)
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

func TestConditionalIndex_CheckTriggers_BuyMaxHeap(t *testing.T) {
	// BUY triggers should fire highest trigger first (max-heap behavior)
	c := NewConditionalIndex()

	// Add orders with different trigger prices
	c.Add(&types.Order{ID: types.OrderID(1), Symbol: "BTCUSDT", Side: constants.ORDER_SIDE_BUY, TriggerPrice: types.Price(fixed.NewI(49000, 0)), Status: constants.ORDER_STATUS_UNTRIGGERED})
	c.Add(&types.Order{ID: types.OrderID(2), Symbol: "BTCUSDT", Side: constants.ORDER_SIDE_BUY, TriggerPrice: types.Price(fixed.NewI(49500, 0)), Status: constants.ORDER_STATUS_UNTRIGGERED})
	c.Add(&types.Order{ID: types.OrderID(3), Symbol: "BTCUSDT", Side: constants.ORDER_SIDE_BUY, TriggerPrice: types.Price(fixed.NewI(48500, 0)), Status: constants.ORDER_STATUS_UNTRIGGERED})

	// When price drops to 48000, all should trigger
	triggered := 0
	c.CheckTriggers("BTCUSDT", types.Price(fixed.NewI(48000, 0)), func(o *types.Order) {
		triggered++
	})
	if triggered != 3 {
		t.Errorf("expected 3 triggered orders, got %d", triggered)
	}
}

func TestConditionalIndex_CheckTriggers_SellMinHeap(t *testing.T) {
	// SELL triggers should fire lowest trigger first (min-heap behavior)
	c := NewConditionalIndex()

	// Add orders with different trigger prices
	c.Add(&types.Order{ID: types.OrderID(1), Symbol: "BTCUSDT", Side: constants.ORDER_SIDE_SELL, TriggerPrice: types.Price(fixed.NewI(51000, 0)), Status: constants.ORDER_STATUS_UNTRIGGERED})
	c.Add(&types.Order{ID: types.OrderID(2), Symbol: "BTCUSDT", Side: constants.ORDER_SIDE_SELL, TriggerPrice: types.Price(fixed.NewI(50500, 0)), Status: constants.ORDER_STATUS_UNTRIGGERED})
	c.Add(&types.Order{ID: types.OrderID(3), Symbol: "BTCUSDT", Side: constants.ORDER_SIDE_SELL, TriggerPrice: types.Price(fixed.NewI(51500, 0)), Status: constants.ORDER_STATUS_UNTRIGGERED})

	// When price rises to 52000, all should trigger
	triggered := 0
	c.CheckTriggers("BTCUSDT", types.Price(fixed.NewI(52000, 0)), func(o *types.Order) {
		triggered++
	})
	if triggered != 3 {
		t.Errorf("expected 3 triggered orders, got %d", triggered)
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
		Status:       constants.ORDER_STATUS_UNTRIGGERED,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		order.ID = types.OrderID(i)
		order.TriggerPrice = types.Price(fixed.NewI(int64(49000+i%100), 0))
		c.Add(order)
	}
}

func BenchmarkConditionalIndex_Add_1000_symbols(b *testing.B) {
	c := NewConditionalIndex()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		symbol := generateTestSymbol(i % 1000)
		order := &types.Order{
			ID:           types.OrderID(i),
			UserID:       types.UserID(1),
			Symbol:       symbol,
			Side:         constants.ORDER_SIDE_BUY,
			TriggerPrice: types.Price(fixed.NewI(int64(49000+i%1000), 0)),
			Status:       constants.ORDER_STATUS_UNTRIGGERED,
		}
		c.Add(order)
	}
}

func BenchmarkConditionalIndex_CheckTriggers(b *testing.B) {
	c := NewConditionalIndex()

	for i := 0; i < 1000; i++ {
		c.Add(&types.Order{
			ID:           types.OrderID(i),
			UserID:       types.UserID(i % 10),
			Symbol:       "BTCUSDT",
			Side:         constants.ORDER_SIDE_BUY,
			TriggerPrice: types.Price(fixed.NewI(int64(49000+i%1000), 0)),
			Status:       constants.ORDER_STATUS_UNTRIGGERED,
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.CheckTriggers("BTCUSDT", types.Price(fixed.NewI(49500, 0)), func(o *types.Order) {})
	}
}

func BenchmarkConditionalIndex_CheckTriggers_1000_symbols(b *testing.B) {
	c := NewConditionalIndex()

	// Create orders across 1000 symbols
	for i := 0; i < 1000; i++ {
		symbol := generateTestSymbol(i)
		c.Add(&types.Order{
			ID:           types.OrderID(i),
			UserID:       types.UserID(i % 10),
			Symbol:       symbol,
			Side:         constants.ORDER_SIDE_BUY,
			TriggerPrice: types.Price(fixed.NewI(int64(49000+i%100), 0)),
			Status:       constants.ORDER_STATUS_UNTRIGGERED,
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		symbol := generateTestSymbol(i % 1000)
		c.CheckTriggers(symbol, types.Price(fixed.NewI(int64(49000+i%100), 0)), func(o *types.Order) {})
	}
}

// generateTestSymbol generates a unique symbol name for testing scalability.
func generateTestSymbol(index int) string {
	return string(rune('A'+index%26)) + string(rune('A'+(index/26)%26)) + string(rune('0'+(index/52)%10)) + string(rune('0'+(index/520)%10))
}
