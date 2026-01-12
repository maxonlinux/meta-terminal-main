package triggers

import (
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func BenchmarkMonitor_Add(b *testing.B) {
	b.ReportAllocs()
	m := NewWithCapacity(b.N)
	order := &types.Order{
		ID:           types.OrderID(1),
		UserID:       types.UserID(1),
		Symbol:       "BTCUSDT",
		Category:     constants.CATEGORY_LINEAR,
		Side:         constants.ORDER_SIDE_BUY,
		Type:         constants.ORDER_TYPE_LIMIT,
		TriggerPrice: types.Price(50000),
		CreatedAt:    types.NowNano(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		order.ID = types.OrderID(uint64(i) + 1)
		m.Add(order)
	}
}

func BenchmarkMonitor_Check(b *testing.B) {
	b.ReportAllocs()
	m := NewWithCapacity(10000)
	orders := make([]*types.Order, 10000)
	for i := 0; i < 10000; i++ {
		orders[i] = &types.Order{
			ID:           types.OrderID(uint64(i) + 1),
			UserID:       types.UserID(1),
			Symbol:       "BTCUSDT",
			Category:     constants.CATEGORY_LINEAR,
			Side:         constants.ORDER_SIDE_BUY,
			Type:         constants.ORDER_TYPE_LIMIT,
			TriggerPrice: types.Price(50000 + i),
			CreatedAt:    types.NowNano(),
		}
	}
	for j := 0; j < 10000; j++ {
		m.Add(orders[j])
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		triggered := m.Check(types.Price(55000))
		for j := 0; j < len(triggered); j++ {
			m.Add(triggered[j])
		}
	}
}

func BenchmarkMonitor_Remove(b *testing.B) {
	b.ReportAllocs()
	m := NewWithCapacity(10000)

	for i := 0; i < 10000; i++ {
		order := &types.Order{
			ID:           types.OrderID(uint64(i) + 1),
			UserID:       types.UserID(1),
			Symbol:       "BTCUSDT",
			Category:     constants.CATEGORY_LINEAR,
			Side:         constants.ORDER_SIDE_BUY,
			Type:         constants.ORDER_TYPE_LIMIT,
			TriggerPrice: types.Price(50000 + i),
			CreatedAt:    types.NowNano(),
		}
		m.Add(order)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Remove(types.OrderID(uint64(i%10000) + 1))
	}
}

func TestMonitor_Count(t *testing.T) {
	m := New()

	if m.Count() != 0 {
		t.Errorf("expected 0, got %d", m.Count())
	}

	order := &types.Order{
		ID:           types.OrderID(1),
		UserID:       types.UserID(1),
		Symbol:       "BTCUSDT",
		Category:     constants.CATEGORY_LINEAR,
		Side:         constants.ORDER_SIDE_BUY,
		Type:         constants.ORDER_TYPE_LIMIT,
		TriggerPrice: types.Price(50000),
		CreatedAt:    types.NowNano(),
	}
	m.Add(order)

	if m.Count() != 1 {
		t.Errorf("expected 1, got %d", m.Count())
	}

	m.Remove(order.ID)

	if m.Count() != 0 {
		t.Errorf("expected 0, got %d", m.Count())
	}
}

func TestMonitor_CheckBuyTriggers(t *testing.T) {
	m := New()

	for i := 0; i < 10; i++ {
		order := &types.Order{
			ID:           types.OrderID(uint64(i) + 1),
			UserID:       types.UserID(1),
			Symbol:       "BTCUSDT",
			Category:     constants.CATEGORY_LINEAR,
			Side:         constants.ORDER_SIDE_BUY,
			Type:         constants.ORDER_TYPE_LIMIT,
			TriggerPrice: types.Price(50000 + i*1000),
			CreatedAt:    types.NowNano(),
		}
		m.Add(order)
	}

	// BUY trigger at P activates when current_price >= P.
	// At current_price = 54000, triggers at 50000-54000 should activate.
	triggered := m.Check(types.Price(54000))

	if len(triggered) != 5 {
		t.Errorf("expected 5 triggered orders (50000-54000), got %d", len(triggered))
	}
}

func TestMonitor_CheckSellTriggers(t *testing.T) {
	m := New()

	for i := 0; i < 10; i++ {
		order := &types.Order{
			ID:           types.OrderID(uint64(i) + 1),
			UserID:       types.UserID(1),
			Symbol:       "BTCUSDT",
			Category:     constants.CATEGORY_LINEAR,
			Side:         constants.ORDER_SIDE_SELL,
			Type:         constants.ORDER_TYPE_LIMIT,
			TriggerPrice: types.Price(55000 - i*1000),
			CreatedAt:    types.NowNano(),
		}
		m.Add(order)
	}

	// SELL trigger at P activates when current_price <= P.
	// At current_price = 54500, only trigger at 55000 should activate (54500 <= 55000).
	triggered := m.Check(types.Price(54500))

	if len(triggered) != 1 {
		t.Errorf("expected 1 triggered order (55000), got %d", len(triggered))
	}
}
