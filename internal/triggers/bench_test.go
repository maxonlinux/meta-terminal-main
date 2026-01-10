package triggers

import (
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func BenchmarkMonitor_Add(b *testing.B) {
	m := New()
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
	m := New()

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
		m.Check(types.Price(55000))
	}
}

func BenchmarkMonitor_Remove(b *testing.B) {
	m := New()

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

func BenchmarkMonitor_AddCheckRemove(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m := New()
		orders := make([]*types.Order, 10000)

		for j := 0; j < 10000; j++ {
			orders[j] = &types.Order{
				ID:           types.OrderID(uint64(i*10000+j) + 1),
				UserID:       types.UserID(1),
				Symbol:       "BTCUSDT",
				Category:     constants.CATEGORY_LINEAR,
				Side:         constants.ORDER_SIDE_BUY,
				Type:         constants.ORDER_TYPE_LIMIT,
				TriggerPrice: types.Price(50000 + j),
				CreatedAt:    uint64(j),
			}
		}

		for j := 0; j < 5000; j++ {
			m.Add(orders[j])
		}
		m.Check(types.Price(52500))
		for j := 0; j < 2500; j++ {
			m.Remove(orders[j].ID)
		}
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

	triggered := m.Check(types.Price(54000))

	if len(triggered) != 5 {
		t.Errorf("expected 5 triggered orders (50000,51000,52000,53000,54000), got %d", len(triggered))
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

	triggered := m.Check(types.Price(54500))

	if len(triggered) != 1 {
		t.Errorf("expected 1 triggered orders (55000 >= 54500), got %d", len(triggered))
	}
}
