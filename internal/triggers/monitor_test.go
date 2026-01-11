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

	triggered := m.Check(100)
	if len(triggered) != 1 || triggered[0].ID != buyOrder.ID {
		t.Fatalf("expected buy trigger, got %v", triggered)
	}

	triggered = m.Check(85)
	if len(triggered) != 0 {
		t.Fatalf("expected no triggers at price 85, got %v", triggered)
	}

	triggered = m.Check(120)
	if len(triggered) != 1 || triggered[0].ID != sellOrder.ID {
		t.Fatalf("expected sell trigger, got %v", triggered)
	}
}

func TestMonitor_StopLossInvertsTriggerSide(t *testing.T) {
	m := New()
	order := &types.Order{
		ID:             1,
		Side:           constants.ORDER_SIDE_SELL,
		TriggerPrice:   100,
		CreatedAt:      1,
		CloseOnTrigger: true,
		StopOrderType:  constants.STOP_ORDER_TYPE_STOP_LOSS,
	}

	m.Add(order)

	if triggered := m.Check(101); len(triggered) != 0 {
		t.Fatalf("expected no triggers at price 101, got %v", triggered)
	}

	triggered := m.Check(99)
	if len(triggered) != 1 || triggered[0].ID != order.ID {
		t.Fatalf("expected stop loss trigger at price 99, got %v", triggered)
	}
}
