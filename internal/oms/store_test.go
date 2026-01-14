package oms

import (
	"testing"

	"github.com/maxonlinux/meta-terminal-go/internal/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

func TestStore_AddOrder(t *testing.T) {
	s := New()

	order := &types.Order{
		UserID:   types.UserID(1),
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		Price:    types.Price(50000),
		Quantity: types.Quantity(10),
	}

	o := s.AddOrder(order)

	if o.ID == 0 {
		t.Error("expected order ID to be set")
	}
	if o.CreatedAt == 0 {
		t.Error("expected CreatedAt to be set")
	}
	if s.Count() != 1 {
		t.Errorf("expected count 1, got %d", s.Count())
	}
}

func TestStore_GetOrder(t *testing.T) {
	s := New()

	order := &types.Order{
		UserID:   types.UserID(1),
		Symbol:   "BTCUSDT",
		Quantity: types.Quantity(10),
	}
	s.AddOrder(order)

	o := s.GetOrder(order.ID)
	if o == nil {
		t.Fatal("expected order, got nil")
	}
	if o.UserID != types.UserID(1) {
		t.Errorf("expected UserID 1, got %d", o.UserID)
	}
}

func TestStore_RemoveOrder(t *testing.T) {
	s := New()

	order := &types.Order{
		UserID:   types.UserID(1),
		Symbol:   "BTCUSDT",
		Quantity: types.Quantity(10),
	}
	s.AddOrder(order)

	if !s.RemoveOrder(order.ID) {
		t.Error("expected RemoveOrder to return true")
	}
	if s.Count() != 0 {
		t.Errorf("expected count 0, got %d", s.Count())
	}
}

func TestStore_ROTrimming(t *testing.T) {
	s := New()

	// Add RO SELL orders at different prices
	s.AddOrder(&types.Order{
		ID:         types.OrderID(1),
		UserID:     types.UserID(1),
		Symbol:     "BTCUSDT",
		Category:   constants.CATEGORY_LINEAR,
		Side:       constants.ORDER_SIDE_SELL,
		Price:      types.Price(55000), // Highest - furthest from market
		Quantity:   types.Quantity(5),
		ReduceOnly: true,
		Status:     constants.ORDER_STATUS_NEW,
	})
	s.AddOrder(&types.Order{
		ID:         types.OrderID(2),
		UserID:     types.UserID(1),
		Symbol:     "BTCUSDT",
		Category:   constants.CATEGORY_LINEAR,
		Side:       constants.ORDER_SIDE_SELL,
		Price:      types.Price(53000), // Lowest - nearest to market
		Quantity:   types.Quantity(5),
		ReduceOnly: true,
		Status:     constants.ORDER_STATUS_NEW,
	})

	// Set position to 5 (can only reduce by 5)
	s.SetPosition(types.UserID(1), "BTCUSDT", 5)

	// Trim should remove highest price first (55000)
	trimmed := s.TrimROOrders(types.UserID(1), "BTCUSDT", 5)

	if trimmed != 5 {
		t.Errorf("expected 5 trimmed, got %d", trimmed)
	}

	// Order at 55000 should be removed
	if o := s.GetOrder(types.OrderID(1)); o != nil && o.Status != constants.ORDER_STATUS_CANCELED {
		t.Error("expected order at 55000 to be removed/canceled")
	}

	// Order at 53000 should remain
	if o := s.GetOrder(types.OrderID(2)); o == nil {
		t.Error("expected order at 53000 to remain")
	}
}

func TestStore_PositionUpdate(t *testing.T) {
	s := New()

	// Set initial position
	if s.SetPosition(1, "BTCUSDT", 10) {
		t.Error("expected no trimming needed for new position")
	}

	// Reduce position - should trigger trimming
	if !s.SetPosition(1, "BTCUSDT", 5) {
		t.Error("expected trimming to be required when position reduced")
	}
}
