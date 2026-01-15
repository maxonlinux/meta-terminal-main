package oms

import (
	"testing"

	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/robaho/fixed"
)

func TestService_Sync_CreateRO(t *testing.T) {
	s := NewService()

	order := s.Create(types.UserID(1), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC,
		types.Price(fixed.NewI(50000, 0)), types.Quantity(fixed.NewI(10, 0)), types.Price(fixed.NewI(0, 0)), true, false, 0)

	if s.reduceonly.exposure["BTCUSDT"][types.UserID(1)].Cmp(types.Quantity(fixed.NewI(10, 0))) != 0 {
		t.Errorf("expected exposure 10, got %d", s.reduceonly.exposure["BTCUSDT"][types.UserID(1)])
	}

	found := false
	h := s.reduceonly.buyHeaps["BTCUSDT"]
	for _, item := range h.items {
		if item.ID == order.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("order not found in buyHeaps")
	}
}

func TestService_Sync_CreateConditional(t *testing.T) {
	s := NewService()

	s.Create(types.UserID(1), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC,
		types.Price(fixed.NewI(50000, 0)), types.Quantity(fixed.NewI(10, 0)), types.Price(fixed.NewI(49000, 0)), false, false, constants.STOP_ORDER_TYPE_STOP)

	if s.conditional.buyTriggers["BTCUSDT"] == nil {
		t.Error("buyTriggers[BTCUSDT] should be created")
	}
}

func TestService_Sync_CancelRO(t *testing.T) {
	s := NewService()

	order := s.Create(types.UserID(1), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC,
		types.Price(fixed.NewI(50000, 0)), types.Quantity(fixed.NewI(10, 0)), types.Price(fixed.NewI(0, 0)), true, false, 0)

	s.Cancel(order.ID)

	if s.reduceonly.exposure["BTCUSDT"][types.UserID(1)].Cmp(types.Quantity(fixed.NewI(0, 0))) != 0 {
		t.Errorf("expected exposure 0, got %d", s.reduceonly.exposure["BTCUSDT"][types.UserID(1)])
	}

	if !s.reduceonly.deleted[&order.ID] {
		t.Error("order should be marked as deleted")
	}
}

func TestService_Sync_CancelConditional(t *testing.T) {
	s := NewService()

	order := s.Create(types.UserID(1), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC,
		types.Price(fixed.NewI(50000, 0)), types.Quantity(fixed.NewI(10, 0)), types.Price(fixed.NewI(49000, 0)), false, false, constants.STOP_ORDER_TYPE_STOP)

	s.Cancel(order.ID)

	if !s.conditional.deleted[&order.ID] {
		t.Error("order should be marked as deleted")
	}
}

// TestService_Sync_CancelThenOnPriceTick verifies that OnPriceTick properly
// cleans up canceled orders from the conditional heap.
func TestService_Sync_CancelThenOnPriceTick(t *testing.T) {
	s := NewService()

	// Create conditional order
	condOrder := s.Create(types.UserID(1), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC,
		types.Price(fixed.NewI(50000, 0)), types.Quantity(fixed.NewI(10, 0)), types.Price(fixed.NewI(49000, 0)), false, false, constants.STOP_ORDER_TYPE_STOP)

	// Cancel it first
	s.Cancel(condOrder.ID)

	// Verify marked as deleted
	if !s.conditional.deleted[&condOrder.ID] {
		t.Error("canceled order should be marked as deleted")
	}

	// OnPriceTick should NOT trigger the canceled order
	// It should clean up the canceled order from the heap
	triggered := false
	s.OnPriceTick("BTCUSDT", types.Price(fixed.NewI(48500, 0)), func(o *types.Order) {
		triggered = true
	})

	// Canceled order should NOT be triggered
	if triggered {
		t.Error("canceled order should not be triggered")
	}

	// Order status should still be CANCELED (not TRIGGERED)
	if condOrder.Status != constants.ORDER_STATUS_CANCELED {
		t.Errorf("expected CANCELED status, got %d", condOrder.Status)
	}
}

func TestService_Sync_FillPartialRO(t *testing.T) {
	s := NewService()

	order := s.Create(types.UserID(1), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC,
		types.Price(fixed.NewI(50000, 0)), types.Quantity(fixed.NewI(10, 0)), types.Price(fixed.NewI(0, 0)), true, false, 0)

	s.Fill(order.ID, types.Quantity(fixed.NewI(5, 0)))

	if s.reduceonly.exposure["BTCUSDT"][types.UserID(1)].Cmp(types.Quantity(fixed.NewI(5, 0))) != 0 {
		t.Errorf("expected exposure 5, got %d", s.reduceonly.exposure["BTCUSDT"][types.UserID(1)])
	}
}

func TestService_Sync_FillFullRO(t *testing.T) {
	s := NewService()

	order := s.Create(types.UserID(1), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC,
		types.Price(fixed.NewI(50000, 0)), types.Quantity(fixed.NewI(10, 0)), types.Price(fixed.NewI(0, 0)), true, false, 0)

	s.Fill(order.ID, types.Quantity(fixed.NewI(10, 0)))

	if s.reduceonly.exposure["BTCUSDT"][types.UserID(1)].Cmp(types.Quantity(fixed.NewI(0, 0))) != 0 {
		t.Errorf("expected exposure 0, got %d", s.reduceonly.exposure["BTCUSDT"][types.UserID(1)])
	}

	if !s.reduceonly.deleted[&order.ID] {
		t.Error("order should be marked as deleted")
	}
}

func TestService_Sync_FillFullConditional(t *testing.T) {
	s := NewService()

	order := s.Create(types.UserID(1), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC,
		types.Price(fixed.NewI(50000, 0)), types.Quantity(fixed.NewI(10, 0)), types.Price(fixed.NewI(49000, 0)), false, false, constants.STOP_ORDER_TYPE_STOP)

	s.Fill(order.ID, types.Quantity(fixed.NewI(10, 0)))

	if !s.conditional.deleted[&order.ID] {
		t.Error("conditional order should be marked as deleted after full fill")
	}
}

func TestService_Sync_AmendRO(t *testing.T) {
	s := NewService()

	order := s.Create(types.UserID(1), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC,
		types.Price(fixed.NewI(50000, 0)), types.Quantity(fixed.NewI(10, 0)), types.Price(fixed.NewI(0, 0)), true, false, 0)

	s.Amend(order.ID, types.Quantity(fixed.NewI(7, 0)))

	if s.reduceonly.exposure["BTCUSDT"][types.UserID(1)].Cmp(types.Quantity(fixed.NewI(7, 0))) != 0 {
		t.Errorf("expected exposure 7, got %d", s.reduceonly.exposure["BTCUSDT"][types.UserID(1)])
	}

	if order.Quantity.Cmp(types.Quantity(fixed.NewI(7, 0))) != 0 {
		t.Errorf("expected quantity 7, got %d", order.Quantity)
	}
}

func TestService_Sync_CancelNonRO(t *testing.T) {
	s := NewService()

	order := s.Create(types.UserID(1), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC,
		types.Price(fixed.NewI(50000, 0)), types.Quantity(fixed.NewI(10, 0)), types.Price(fixed.NewI(0, 0)), false, false, 0)

	s.Cancel(order.ID)

	if s.reduceonly.buyHeaps["BTCUSDT"] != nil {
		t.Error("non-RO order should not be in reduceonly index")
	}
}

func TestService_Sync_FillNonRO(t *testing.T) {
	s := NewService()

	// Create non-reduceOnly order
	s.Create(types.UserID(1), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC,
		types.Price(fixed.NewI(50000, 0)), types.Quantity(fixed.NewI(10, 0)), types.Price(fixed.NewI(0, 0)), false, false, 0)

	// Fill the order
	s.Fill(types.OrderID(1), types.Quantity(fixed.NewI(5, 0)))

	// Exposure map should not exist for non-RO orders
	if s.reduceonly.exposure["BTCUSDT"] != nil {
		t.Error("non-RO order should not have exposure entry")
	}
}

func TestService_Sync_CancelNonConditional(t *testing.T) {
	s := NewService()

	order := s.Create(types.UserID(1), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC,
		types.Price(fixed.NewI(50000, 0)), types.Quantity(fixed.NewI(10, 0)), types.Price(fixed.NewI(0, 0)), false, false, 0)

	s.Cancel(order.ID)

	if s.conditional.buyTriggers["BTCUSDT"] != nil {
		t.Error("non-conditional order should not be in conditional index")
	}
}

// TestService_Sync_Full_Fill_RO_Conditional verifies complete synchronization
// when an order is both reduceOnly and conditional, and gets fully filled.
func TestService_Sync_Full_Fill_RO_Conditional(t *testing.T) {
	s := NewService()

	// Create order that is both reduceOnly and conditional
	order := s.Create(types.UserID(1), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC,
		types.Price(fixed.NewI(50000, 0)), types.Quantity(fixed.NewI(10, 0)), types.Price(fixed.NewI(49000, 0)), true, false, constants.STOP_ORDER_TYPE_STOP)

	// Verify order is in both indices
	if s.reduceonly.exposure["BTCUSDT"][types.UserID(1)].Cmp(types.Quantity(fixed.NewI(10, 0))) != 0 {
		t.Errorf("expected initial exposure 10")
	}

	if s.conditional.buyTriggers["BTCUSDT"] == nil {
		t.Error("order should be in conditional index")
	}

	// Full fill
	s.Fill(order.ID, types.Quantity(fixed.NewI(10, 0)))

	// Verify order status
	if order.Status != constants.ORDER_STATUS_FILLED {
		t.Errorf("expected FILLED, got %d", order.Status)
	}

	// Verify exposure reduced to 0
	if s.reduceonly.exposure["BTCUSDT"][types.UserID(1)].Cmp(types.Quantity(fixed.NewI(0, 0))) != 0 {
		t.Errorf("expected exposure 0 after full fill, got %d", s.reduceonly.exposure["BTCUSDT"][types.UserID(1)])
	}

	// Verify order removed from both indices
	if !s.reduceonly.deleted[&order.ID] {
		t.Error("order should be marked as deleted in reduceonly index")
	}

	if !s.conditional.deleted[&order.ID] {
		t.Error("order should be marked as deleted in conditional index")
	}

	// Verify order still exists in main map (for history)
	if _, ok := s.Get(order.ID); !ok {
		t.Error("order should still exist in main map")
	}
}

// TestService_Sync_Amend_After_PartialFill_RO verifies exposure recalculation
// when amending a partially filled reduceOnly order.
func TestService_Sync_Amend_After_PartialFill_RO(t *testing.T) {
	s := NewService()

	// Create reduceOnly order with qty 10
	order := s.Create(types.UserID(1), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC,
		types.Price(fixed.NewI(50000, 0)), types.Quantity(fixed.NewI(10, 0)), types.Price(fixed.NewI(0, 0)), true, false, 0)

	// Partial fill 3
	s.Fill(order.ID, types.Quantity(fixed.NewI(3, 0)))

	// Exposure should be 7 (10 - 3 filled)
	if s.reduceonly.exposure["BTCUSDT"][types.UserID(1)].Cmp(types.Quantity(fixed.NewI(7, 0))) != 0 {
		t.Errorf("expected exposure 7 after partial fill, got %d", s.reduceonly.exposure["BTCUSDT"][types.UserID(1)])
	}

	// Amend remaining qty from 7 to 5 (reduce by 2 more)
	s.Amend(order.ID, types.Quantity(fixed.NewI(5, 0)))

	// Exposure should now be 5 (remaining qty)
	if s.reduceonly.exposure["BTCUSDT"][types.UserID(1)].Cmp(types.Quantity(fixed.NewI(5, 0))) != 0 {
		t.Errorf("expected exposure 5 after amend, got %d", s.reduceonly.exposure["BTCUSDT"][types.UserID(1)])
	}

	// Order quantity should be 5
	if order.Quantity.Cmp(types.Quantity(fixed.NewI(5, 0))) != 0 {
		t.Errorf("expected order quantity 5, got %d", order.Quantity)
	}
}

// TestService_Sync_Cancel_RO_After_PartialFill verifies exposure cleanup
// when canceling a partially filled reduceOnly order.
func TestService_Sync_Cancel_RO_After_PartialFill(t *testing.T) {
	s := NewService()

	// Create reduceOnly order with qty 10
	order := s.Create(types.UserID(1), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC,
		types.Price(fixed.NewI(50000, 0)), types.Quantity(fixed.NewI(10, 0)), types.Price(fixed.NewI(0, 0)), true, false, 0)

	// Partial fill 3
	s.Fill(order.ID, types.Quantity(fixed.NewI(3, 0)))

	// Cancel
	s.Cancel(order.ID)

	// Order status should be PARTIALLY_FILLED_CANCELED
	if order.Status != constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED {
		t.Errorf("expected PARTIALLY_FILLED_CANCELED, got %d", order.Status)
	}

	// Exposure should be 0 (removed from reduceOnly index)
	if s.reduceonly.exposure["BTCUSDT"][types.UserID(1)].Cmp(types.Quantity(fixed.NewI(0, 0))) != 0 {
		t.Errorf("expected exposure 0 after cancel, got %d", s.reduceonly.exposure["BTCUSDT"][types.UserID(1)])
	}

	// Order should be marked as deleted in reduceOnly index
	if !s.reduceonly.deleted[&order.ID] {
		t.Error("order should be marked as deleted in reduceonly index")
	}
}

// TestService_Sync_Multiple_RO_Orders_Aggregate verifies that exposure is correctly
// aggregated when a user has multiple reduceOnly orders for the same symbol.
func TestService_Sync_Multiple_RO_Orders_Aggregate(t *testing.T) {
	s := NewService()

	// Create 3 reduceOnly orders for same user/symbol: qty 10, 5, 15
	order1 := s.Create(types.UserID(1), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC,
		types.Price(fixed.NewI(50000, 0)), types.Quantity(fixed.NewI(10, 0)), types.Price(fixed.NewI(0, 0)), true, false, 0)

	order2 := s.Create(types.UserID(1), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC,
		types.Price(fixed.NewI(50000, 0)), types.Quantity(fixed.NewI(5, 0)), types.Price(fixed.NewI(0, 0)), true, false, 0)

	order3 := s.Create(types.UserID(1), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC,
		types.Price(fixed.NewI(50000, 0)), types.Quantity(fixed.NewI(15, 0)), types.Price(fixed.NewI(0, 0)), true, false, 0)

	// Verify aggregated exposure = 10 + 5 + 15 = 30
	if s.reduceonly.exposure["BTCUSDT"][types.UserID(1)].Cmp(types.Quantity(fixed.NewI(30, 0))) != 0 {
		t.Errorf("expected aggregated exposure 30, got %d", s.reduceonly.exposure["BTCUSDT"][types.UserID(1)])
	}

	// Fill first order partially (5 out of 10)
	s.Fill(order1.ID, types.Quantity(fixed.NewI(5, 0)))

	// Exposure should be 25 (30 - 5)
	if s.reduceonly.exposure["BTCUSDT"][types.UserID(1)].Cmp(types.Quantity(fixed.NewI(25, 0))) != 0 {
		t.Errorf("expected exposure 25 after partial fill, got %d", s.reduceonly.exposure["BTCUSDT"][types.UserID(1)])
	}

	// Fully fill second order
	s.Fill(order2.ID, types.Quantity(fixed.NewI(5, 0)))

	// Exposure should be 20 (25 - 5)
	if s.reduceonly.exposure["BTCUSDT"][types.UserID(1)].Cmp(types.Quantity(fixed.NewI(20, 0))) != 0 {
		t.Errorf("expected exposure 20 after full fill of order2, got %d", s.reduceonly.exposure["BTCUSDT"][types.UserID(1)])
	}

	// Verify order2 is marked as deleted
	if !s.reduceonly.deleted[&order2.ID] {
		t.Error("order2 should be marked as deleted after full fill")
	}

	// Verify order1 and order3 still in index (not deleted)
	if s.reduceonly.deleted[&order1.ID] {
		t.Error("order1 should NOT be deleted (partial fill)")
	}
	if s.reduceonly.deleted[&order3.ID] {
		t.Error("order3 should NOT be deleted (not filled)")
	}
}

// TestService_Sync_Trigger_Creates_ChildOrder verifies the complete trigger chain:
// conditional order triggers -> popped from heap -> status = TRIGGERED.
// Order is NOT marked as deleted in conditional index (removed directly via Pop).
func TestService_Sync_Trigger_Creates_ChildOrder(t *testing.T) {
	s := NewService()
	triggered := false
	var triggeredOrder *types.Order

	// Create conditional order
	condOrder := s.Create(types.UserID(1), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC,
		types.Price(fixed.NewI(50000, 0)), types.Quantity(fixed.NewI(10, 0)), types.Price(fixed.NewI(49000, 0)), false, false, constants.STOP_ORDER_TYPE_STOP)

	// Verify conditional order is in buyTriggers
	if s.conditional.buyTriggers["BTCUSDT"] == nil {
		t.Fatal("conditional order should be in buyTriggers")
	}

	// Verify NOT in reduceonly (it's not reduceOnly)
	if s.reduceonly.buyHeaps["BTCUSDT"] != nil {
		t.Error("non-RO order should not be in reduceonly index")
	}

	// Price tick that triggers the order
	s.OnPriceTick("BTCUSDT", types.Price(fixed.NewI(48500, 0)), func(o *types.Order) {
		triggered = true
		triggeredOrder = o
	})

	// Verify trigger fired
	if !triggered {
		t.Fatal("order should have been triggered")
	}

	// Verify triggered order has correct properties
	if triggeredOrder == nil {
		t.Fatal("triggered order should be received")
	}

	// Order should have status TRIGGERED
	if triggeredOrder.Status != constants.ORDER_STATUS_TRIGGERED {
		t.Errorf("expected TRIGGERED status, got %d", triggeredOrder.Status)
	}

	// Order should be in main map and retrievable
	if _, ok := s.Get(condOrder.ID); !ok {
		t.Error("triggered order should be in main map")
	}

	// Verify the triggered order IS the original order
	if triggeredOrder.ID != condOrder.ID {
		t.Error("triggered order should be the original order")
	}
}

// TestService_Sync_Trigger_And_Fill verifies that a triggered conditional order
// can be filled normally.
func TestService_Sync_Trigger_And_Fill(t *testing.T) {
	s := NewService()

	// Create conditional order
	condOrder := s.Create(types.UserID(1), "BTCUSDT", constants.CATEGORY_LINEAR,
		constants.ORDER_SIDE_BUY, constants.ORDER_TYPE_LIMIT, constants.TIF_GTC,
		types.Price(fixed.NewI(49000, 0)), types.Quantity(fixed.NewI(10, 0)), types.Price(fixed.NewI(48500, 0)), false, false, constants.STOP_ORDER_TYPE_STOP)

	// Trigger the order
	s.OnPriceTick("BTCUSDT", types.Price(fixed.NewI(48500, 0)), func(o *types.Order) {})

	// Verify it's triggered
	if condOrder.Status != constants.ORDER_STATUS_TRIGGERED {
		t.Errorf("expected TRIGGERED status, got %d", condOrder.Status)
	}

	// Fill the triggered order
	s.Fill(condOrder.ID, types.Quantity(fixed.NewI(10, 0)))

	// Verify filled status
	if condOrder.Status != constants.ORDER_STATUS_FILLED {
		t.Errorf("expected FILLED status after fill, got %d", condOrder.Status)
	}

	// Verify filled amount
	if condOrder.Filled.Cmp(types.Quantity(fixed.NewI(10, 0))) != 0 {
		t.Errorf("expected filled 10, got %d", condOrder.Filled)
	}
}
