package trigger

import (
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func TestMonitor_AddOrder(t *testing.T) {
	s := state.New()
	m := NewMonitor(s)

	order := &types.Order{
		ID:             1,
		UserID:         100,
		Symbol:         1,
		Side:           constants.ORDER_SIDE_BUY,
		Type:           constants.ORDER_TYPE_LIMIT,
		TIF:            constants.TIF_GTC,
		Price:          50000,
		Quantity:       1,
		Status:         constants.ORDER_STATUS_UNTRIGGERED,
		TriggerPrice:   49000,
		StopOrderType:  constants.STOP_ORDER_TYPE_STOP,
		ReduceOnly:     false,
		CloseOnTrigger: false,
	}

	m.AddOrder(order)

	ss := s.GetSymbolState(1)
	if ss.BuyTriggers == nil {
		t.Fatal("expected BuyTriggers to be set")
	}
	if ss.BuyTriggers.Len() != 1 {
		t.Errorf("expected heap length 1, got %d", ss.BuyTriggers.Len())
	}
}

func TestMonitor_Check_BuyTrigger(t *testing.T) {
	s := state.New()
	m := NewMonitor(s)

	order := &types.Order{
		ID:             1,
		UserID:         100,
		Symbol:         1,
		Side:           constants.ORDER_SIDE_BUY,
		Type:           constants.ORDER_TYPE_LIMIT,
		TIF:            constants.TIF_GTC,
		Price:          50000,
		Quantity:       1,
		Status:         constants.ORDER_STATUS_UNTRIGGERED,
		TriggerPrice:   49000,
		StopOrderType:  constants.STOP_ORDER_TYPE_STOP,
		ReduceOnly:     false,
		CloseOnTrigger: false,
	}

	m.AddOrder(order)

	triggered := m.Check(1, 48500)
	if len(triggered) != 0 {
		t.Errorf("expected 0 triggered orders, got %d", len(triggered))
	}

	triggered = m.Check(1, 49000)
	if len(triggered) != 1 {
		t.Errorf("expected 1 triggered order, got %d", len(triggered))
	}
	if triggered[0].ID != 1 {
		t.Errorf("expected order ID 1, got %d", triggered[0].ID)
	}

	triggered = m.Check(1, 50000)
	if len(triggered) != 0 {
		t.Errorf("expected 0 triggered orders (already triggered), got %d", len(triggered))
	}
}

func TestMonitor_Check_SellTrigger(t *testing.T) {
	s := state.New()
	m := NewMonitor(s)

	order := &types.Order{
		ID:             1,
		UserID:         100,
		Symbol:         1,
		Side:           constants.ORDER_SIDE_SELL,
		Type:           constants.ORDER_TYPE_LIMIT,
		TIF:            constants.TIF_GTC,
		Price:          48000,
		Quantity:       1,
		Status:         constants.ORDER_STATUS_UNTRIGGERED,
		TriggerPrice:   50000,
		StopOrderType:  constants.STOP_ORDER_TYPE_STOP,
		ReduceOnly:     false,
		CloseOnTrigger: false,
	}

	m.AddOrder(order)

	triggered := m.Check(1, 51000)
	if len(triggered) != 0 {
		t.Errorf("expected 0 triggered orders, got %d", len(triggered))
	}

	triggered = m.Check(1, 50000)
	if len(triggered) != 1 {
		t.Errorf("expected 1 triggered order, got %d", len(triggered))
	}

	triggered = m.Check(1, 49000)
	if len(triggered) != 0 {
		t.Errorf("expected 0 triggered orders (already triggered), got %d", len(triggered))
	}
}

func TestMonitor_Check_MultipleOrders(t *testing.T) {
	s := state.New()
	m := NewMonitor(s)

	orders := []*types.Order{
		{ID: 1, UserID: 100, Symbol: 1, Side: constants.ORDER_SIDE_BUY, TriggerPrice: 47000, StopOrderType: constants.STOP_ORDER_TYPE_STOP},
		{ID: 2, UserID: 100, Symbol: 1, Side: constants.ORDER_SIDE_BUY, TriggerPrice: 48000, StopOrderType: constants.STOP_ORDER_TYPE_STOP},
		{ID: 3, UserID: 100, Symbol: 1, Side: constants.ORDER_SIDE_BUY, TriggerPrice: 49000, StopOrderType: constants.STOP_ORDER_TYPE_STOP},
	}

	for _, o := range orders {
		m.AddOrder(o)
	}

	triggered := m.Check(1, 48500)
	if len(triggered) != 2 {
		t.Errorf("expected 2 triggered orders, got %d", len(triggered))
	}

	ss := s.GetSymbolState(1)
	if ss.BuyTriggers.Len() != 1 {
		t.Errorf("expected 1 remaining in heap, got %d", ss.BuyTriggers.Len())
	}
}

func TestMonitor_OnTrigger_CloseOnTrigger(t *testing.T) {
	s := state.New()
	m := NewMonitor(s)

	order := &types.Order{
		ID:             1,
		UserID:         100,
		Symbol:         1,
		Side:           constants.ORDER_SIDE_BUY,
		TriggerPrice:   49000,
		StopOrderType:  constants.STOP_ORDER_TYPE_STOP,
		CloseOnTrigger: true,
	}

	result, err := m.OnTrigger(order)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result for CloseOnTrigger, got %v", result)
	}
	if order.Status != constants.ORDER_STATUS_TRIGGERED {
		t.Errorf("expected status TRIGGERED, got %d", order.Status)
	}
}

func TestMonitor_OnTrigger_CreateNewOrder(t *testing.T) {
	s := state.New()
	m := NewMonitor(s)

	initialOrderID := s.NextOrderID

	order := &types.Order{
		ID:             1,
		UserID:         100,
		Symbol:         1,
		Side:           constants.ORDER_SIDE_BUY,
		Type:           constants.ORDER_TYPE_LIMIT,
		TIF:            constants.TIF_GTC,
		Price:          50000,
		Quantity:       1,
		TriggerPrice:   49000,
		StopOrderType:  constants.STOP_ORDER_TYPE_STOP,
		CloseOnTrigger: false,
		ReduceOnly:     true,
	}

	result, err := m.OnTrigger(order)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 order, got %d", len(result))
	}

	triggered := result[0]
	if triggered.ID != initialOrderID {
		t.Errorf("expected new order ID %d, got %d", initialOrderID, triggered.ID)
	}
	if triggered.Status != constants.ORDER_STATUS_NEW {
		t.Errorf("expected status NEW, got %d", triggered.Status)
	}
	if triggered.TriggerPrice != 0 {
		t.Errorf("expected TriggerPrice 0, got %d", triggered.TriggerPrice)
	}
	if triggered.StopOrderType != constants.STOP_ORDER_TYPE_NORMAL {
		t.Errorf("expected StopOrderType NORMAL, got %d", triggered.StopOrderType)
	}
	if !triggered.ReduceOnly {
		t.Error("expected ReduceOnly to be preserved")
	}
	if triggered.CloseOnTrigger {
		t.Error("expected CloseOnTrigger to be false")
	}
}

func TestMonitor_RemoveOrder(t *testing.T) {
	s := state.New()
	m := NewMonitor(s)

	order := &types.Order{
		ID:            1,
		UserID:        100,
		Symbol:        1,
		Side:          constants.ORDER_SIDE_BUY,
		TriggerPrice:  49000,
		StopOrderType: constants.STOP_ORDER_TYPE_STOP,
	}

	m.AddOrder(order)

	ss := s.GetSymbolState(1)
	if ss.BuyTriggers.Len() != 1 {
		t.Errorf("expected 1 order in heap, got %d", ss.BuyTriggers.Len())
	}

	m.RemoveOrder(1, 1)

	if ss.BuyTriggers.Len() != 0 {
		t.Errorf("expected 0 orders in heap, got %d", ss.BuyTriggers.Len())
	}
}

func TestMonitor_TP_SL_Orders(t *testing.T) {
	s := state.New()
	m := NewMonitor(s)

	tpOrder := &types.Order{
		ID:            1,
		UserID:        100,
		Symbol:        1,
		Side:          constants.ORDER_SIDE_SELL,
		TriggerPrice:  55000,
		StopOrderType: constants.STOP_ORDER_TYPE_TP,
	}

	slOrder := &types.Order{
		ID:            2,
		UserID:        100,
		Symbol:        1,
		Side:          constants.ORDER_SIDE_BUY,
		TriggerPrice:  45000,
		StopOrderType: constants.STOP_ORDER_TYPE_SL,
	}

	m.AddOrder(tpOrder)
	m.AddOrder(slOrder)

	ss := s.GetSymbolState(1)
	if ss.SellTriggers == nil {
		t.Fatal("expected SellTriggers to be set for TP")
	}
	if ss.BuyTriggers == nil {
		t.Fatal("expected BuyTriggers to be set for SL")
	}
}
