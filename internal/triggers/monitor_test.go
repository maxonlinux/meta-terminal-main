package triggers

import (
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func TestMonitor_CheckTriggers(t *testing.T) {
	m := New()

	buyOrder := &types.Order{
		ID:           1,
		Side:         constants.ORDER_SIDE_BUY,
		TriggerPrice: 100,
		CreatedAt:    1,
	}
	sellOrder := &types.Order{
		ID:           2,
		Side:         constants.ORDER_SIDE_SELL,
		TriggerPrice: 120,
		CreatedAt:    2,
	}

	m.Add(buyOrder)
	m.Add(sellOrder)

	// At price 100: BUY@100 triggers (100>=100), SELL@120 triggers (100<=120)
	triggered := m.Check(100)
	if len(triggered) != 2 {
		t.Fatalf("expected 2 triggers at price 100, got %v", triggered)
	}

	// Re-add orders for next test (Check removes triggered orders)
	m.Add(buyOrder)
	m.Add(sellOrder)

	// At price 85: BUY@100 doesn't trigger (85<100), SELL@120 triggers (85<=120)
	triggered = m.Check(85)
	if len(triggered) != 1 || triggered[0].ID != sellOrder.ID {
		t.Fatalf("expected sell trigger at price 85, got %v", triggered)
	}

	// Re-add orders for next test
	m.Add(buyOrder)
	m.Add(sellOrder)

	// At price 120: both trigger (120>=100 and 120<=120)
	triggered = m.Check(120)
	if len(triggered) != 2 {
		t.Fatalf("expected 2 triggers at price 120, got %v", triggered)
	}
}

func TestMonitor_StopLossInvertsTriggerSide(t *testing.T) {
	m := New()
	// SELL stop-loss with CloseOnTrigger - side is NOT inverted
	// Order is already in correct direction (closing long position)
	order := &types.Order{
		ID:             1,
		Side:           constants.ORDER_SIDE_SELL,
		TriggerPrice:   100,
		CreatedAt:      1,
		CloseOnTrigger: true,
		StopOrderType:  constants.STOP_ORDER_TYPE_STOP_LOSS,
	}

	m.Add(order)

	// SELL at 100: triggers when price <= 100
	// At price 99: 99 <= 100 = TRUE, should trigger
	if triggered := m.Check(99); len(triggered) != 1 {
		t.Fatalf("expected trigger at price 99, got %v", triggered)
	}

	// At price 101: 101 <= 100 = FALSE, shouldn't trigger
	triggered := m.Check(101)
	if len(triggered) != 0 {
		t.Fatalf("expected no triggers at price 101, got %v", triggered)
	}
}
