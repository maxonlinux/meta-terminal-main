package trigger

import (
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func TestMonitorAdd(t *testing.T) {
	m := NewMonitor()
	m.Add(1, constants.ORDER_SIDE_BUY, 49000)
	if m.buy.Len() != 1 {
		t.Fatalf("expected buy heap len 1, got %d", m.buy.Len())
	}
}

func TestMonitorCheckBuy(t *testing.T) {
	m := NewMonitor()
	m.Add(1, constants.ORDER_SIDE_BUY, 49000)

	triggered := m.Check(48500)
	if len(triggered) != 1 || triggered[0] != types.OrderID(1) {
		t.Fatalf("expected order 1 triggered")
	}

	triggered = m.Check(50000)
	if len(triggered) != 0 {
		t.Fatalf("expected no more triggers")
	}
}

func TestMonitorCheckSell(t *testing.T) {
	m := NewMonitor()
	m.Add(2, constants.ORDER_SIDE_SELL, 50000)

	triggered := m.Check(51000)
	if len(triggered) != 1 || triggered[0] != types.OrderID(2) {
		t.Fatalf("expected order 2 triggered")
	}
}

func TestMonitorMultiple(t *testing.T) {
	m := NewMonitor()
	m.Add(1, constants.ORDER_SIDE_BUY, 47000)
	m.Add(2, constants.ORDER_SIDE_BUY, 48000)
	m.Add(3, constants.ORDER_SIDE_BUY, 49000)

	triggered := m.Check(48500)
	if len(triggered) != 1 {
		t.Fatalf("expected 1 triggered, got %d", len(triggered))
	}
	if m.buy.Len() != 2 {
		t.Fatalf("expected 2 remaining in buy heap, got %d", m.buy.Len())
	}
}

func TestMonitorRemove(t *testing.T) {
	m := NewMonitor()
	m.Add(1, constants.ORDER_SIDE_BUY, 49000)
	if m.buy.Len() != 1 {
		t.Fatalf("expected buy heap len 1")
	}
	m.Remove(1)
	if m.buy.Len() != 0 {
		t.Fatalf("expected buy heap len 0")
	}
}
