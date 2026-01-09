package triggers

import (
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func BenchmarkTriggerMonitor_Add(b *testing.B) {
	m := New()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		order := &types.Order{
			ID:           types.OrderID(i + 1),
			UserID:       uint64(i % 1000),
			Symbol:       "BTCUSDT",
			Category:     constants.CATEGORY_LINEAR,
			Side:         constants.ORDER_SIDE_BUY,
			TriggerPrice: types.Price(50000 + i%1000),
			Quantity:     types.Quantity(100),
			Price:        types.Price(50000),
			CreatedAt:    types.NowNano(),
		}
		m.Add(order)
	}
}

func BenchmarkTriggerMonitor_Check(b *testing.B) {
	m := New()

	for i := 0; i < 5000; i++ {
		order := &types.Order{
			ID:           types.OrderID(i + 1),
			UserID:       uint64(i % 1000),
			Symbol:       "BTCUSDT",
			Category:     constants.CATEGORY_LINEAR,
			Side:         constants.ORDER_SIDE_BUY,
			TriggerPrice: types.Price(50000 + i%1000),
			Quantity:     types.Quantity(100),
			Price:        types.Price(50000),
			CreatedAt:    types.NowNano(),
		}
		m.Add(order)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		m.Check(types.Price(50500))
	}
}

func BenchmarkTriggerMonitor_CheckEmpty(b *testing.B) {
	m := New()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		m.Check(types.Price(50500))
	}
}

func BenchmarkTriggerMonitor_GetOrder(b *testing.B) {
	m := New()

	for i := 0; i < 1000; i++ {
		order := &types.Order{
			ID:           types.OrderID(i + 1),
			UserID:       uint64(i % 1000),
			Symbol:       "BTCUSDT",
			Category:     constants.CATEGORY_LINEAR,
			Side:         constants.ORDER_SIDE_BUY,
			TriggerPrice: types.Price(50000 + i%1000),
			Quantity:     types.Quantity(100),
			Price:        types.Price(50000),
			CreatedAt:    types.NowNano(),
		}
		m.Add(order)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		m.GetOrder(types.OrderID(i%1000 + 1))
	}
}

func BenchmarkTriggerMonitor_BuySell(b *testing.B) {
	m := New()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		idx := i % 2000
		order := &types.Order{
			ID:           types.OrderID(i + 1),
			UserID:       uint64(i % 1000),
			Symbol:       "BTCUSDT",
			Category:     constants.CATEGORY_LINEAR,
			Side:         constants.ORDER_SIDE_BUY + int8(idx%2),
			TriggerPrice: types.Price(50000 + idx),
			Quantity:     types.Quantity(100),
			Price:        types.Price(50000),
			CreatedAt:    types.NowNano(),
		}
		m.Add(order)
	}
}

func TestTriggerMonitor_AddRemove(t *testing.T) {
	m := New()

	order := &types.Order{
		ID:           types.OrderID(1),
		UserID:       100,
		Symbol:       "BTCUSDT",
		Category:     constants.CATEGORY_LINEAR,
		Side:         constants.ORDER_SIDE_BUY,
		TriggerPrice: types.Price(50000),
		Quantity:     types.Quantity(100),
		Price:        types.Price(50000),
		CreatedAt:    types.NowNano(),
	}

	m.Add(order)

	if m.Count() != 1 {
		t.Errorf("expected count 1, got %d", m.Count())
	}

	if m.BuyCount() != 1 {
		t.Errorf("expected buy count 1, got %d", m.BuyCount())
	}

	m.Remove(types.OrderID(1))

	if m.Count() != 0 {
		t.Errorf("expected count 0, got %d", m.Count())
	}
}

func TestTriggerMonitor_CheckReturnsTriggered(t *testing.T) {
	m := New()

	for i := 0; i < 100; i++ {
		order := &types.Order{
			ID:           types.OrderID(i + 1),
			UserID:       uint64(i),
			Symbol:       "BTCUSDT",
			Category:     constants.CATEGORY_LINEAR,
			Side:         constants.ORDER_SIDE_BUY,
			TriggerPrice: types.Price(50000 + i),
			Quantity:     types.Quantity(100),
			Price:        types.Price(50000),
			CreatedAt:    types.NowNano(),
		}
		m.Add(order)
	}

	triggered := m.Check(types.Price(50505))

	if len(triggered) != 100 {
		t.Errorf("expected 100 triggered orders, got %d", len(triggered))
	}

	for _, id := range triggered {
		if id < 1 || id > 100 {
			t.Errorf("unexpected order ID %d", id)
		}
	}
}

func TestTriggerMonitor_CheckRemovesFromHeap(t *testing.T) {
	m := New()

	for i := 0; i < 10; i++ {
		order := &types.Order{
			ID:           types.OrderID(i + 1),
			UserID:       uint64(i),
			Symbol:       "BTCUSDT",
			Category:     constants.CATEGORY_LINEAR,
			Side:         constants.ORDER_SIDE_BUY,
			TriggerPrice: types.Price(50500 + i),
			Quantity:     types.Quantity(100),
			Price:        types.Price(50000),
			CreatedAt:    types.NowNano(),
		}
		m.Add(order)
	}

	triggered := m.Check(types.Price(50505))

	if len(triggered) != 6 {
		t.Errorf("expected 6 triggered orders, got %d", len(triggered))
	}

	if m.BuyCount() != 4 {
		t.Errorf("expected 4 remaining in heap, got %d", m.BuyCount())
	}
}

func TestTriggerMonitor_SellSide(t *testing.T) {
	m := New()

	for i := 0; i < 10; i++ {
		order := &types.Order{
			ID:           types.OrderID(i + 1),
			UserID:       uint64(i),
			Symbol:       "BTCUSDT",
			Category:     constants.CATEGORY_LINEAR,
			Side:         constants.ORDER_SIDE_SELL,
			TriggerPrice: types.Price(51000 - i),
			Quantity:     types.Quantity(100),
			Price:        types.Price(50000),
			CreatedAt:    types.NowNano(),
		}
		m.Add(order)
	}

	triggered := m.Check(types.Price(50995))

	if len(triggered) != 6 {
		t.Errorf("expected 6 triggered orders, got %d", len(triggered))
	}
}

func TestTriggerMonitor_MixedSides(t *testing.T) {
	m := New()

	for i := 0; i < 10; i++ {
		var side int8 = constants.ORDER_SIDE_BUY
		if i%2 == 0 {
			side = constants.ORDER_SIDE_SELL
		}
		order := &types.Order{
			ID:           types.OrderID(i + 1),
			UserID:       uint64(i),
			Symbol:       "BTCUSDT",
			Category:     constants.CATEGORY_LINEAR,
			Side:         side,
			TriggerPrice: types.Price(50000 + i%5*100),
			Quantity:     types.Quantity(100),
			Price:        types.Price(50000),
			CreatedAt:    types.NowNano(),
		}
		m.Add(order)
	}

	triggered := m.Check(types.Price(50200))

	if len(triggered) != 6 {
		t.Errorf("expected 6 triggered orders, got %d", len(triggered))
	}
}

func TestTriggerMonitor_RemoveNonExistent(t *testing.T) {
	m := New()

	if m.Remove(types.OrderID(999)) {
		t.Errorf("expected false when removing non-existent order")
	}
}

func TestTriggerMonitor_GetOrderNotFound(t *testing.T) {
	m := New()

	if m.GetOrder(types.OrderID(999)) != nil {
		t.Errorf("expected nil when getting non-existent order")
	}
}

func TestTriggerMonitor_Counts(t *testing.T) {
	m := New()

	if m.Count() != 0 {
		t.Errorf("expected count 0, got %d", m.Count())
	}

	for i := 0; i < 10; i++ {
		var side int8 = constants.ORDER_SIDE_BUY
		if i%2 == 0 {
			side = constants.ORDER_SIDE_SELL
		}
		order := &types.Order{
			ID:           types.OrderID(i + 1),
			UserID:       uint64(i),
			Symbol:       "BTCUSDT",
			Category:     constants.CATEGORY_LINEAR,
			Side:         side,
			TriggerPrice: types.Price(50000 + i%5*100),
			Quantity:     types.Quantity(100),
			Price:        types.Price(50000),
			CreatedAt:    types.NowNano(),
		}
		m.Add(order)
	}

	if m.Count() != 10 {
		t.Errorf("expected count 10, got %d", m.Count())
	}

	if m.BuyCount() != 5 {
		t.Errorf("expected buy count 5, got %d", m.BuyCount())
	}

	if m.SellCount() != 5 {
		t.Errorf("expected sell count 5, got %d", m.SellCount())
	}
}
