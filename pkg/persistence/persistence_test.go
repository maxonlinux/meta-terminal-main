package persistence

import (
	"os"
	"testing"

	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/robaho/fixed"
)

func TestPebbleKVRecovery(t *testing.T) {
	path := t.TempDir()

	store1, err := Open(path)
	if err != nil {
		t.Fatalf("open pebblekv: %v", err)
	}

	for i := 0; i < 100; i++ {
		order := &types.Order{
			ID:       types.OrderID(i + 1),
			UserID:   types.UserID(1),
			Symbol:   "BTC-USD-PERP",
			Side:     constants.ORDER_SIDE_BUY,
			Category: constants.CATEGORY_LINEAR,
			Type:     constants.ORDER_TYPE_LIMIT,
			Quantity: fixed.NewI(1, 8),
			Price:    fixed.NewI(50000, 8),
		}
		if err := store1.PutOrder(order); err != nil {
			t.Fatalf("put order %d: %v", i, err)
		}
	}

	if err := store1.Close(); err != nil {
		t.Fatalf("close store1: %v", err)
	}

	store2, err := Open(path)
	if err != nil {
		t.Fatalf("reopen pebblekv: %v", err)
	}
	defer store2.Close()

	count := 0
	err = store2.RangeOrders(func(order *types.Order) bool {
		count++
		if order.ID == types.OrderID(0) {
			t.Error("invalid order ID 0")
		}
		return true
	})
	if err != nil {
		t.Fatalf("range orders: %v", err)
	}
	if count != 100 {
		t.Errorf("expected 100 orders after recovery, got %d", count)
	}
}

func TestPebbleKVRecoveryUpdate(t *testing.T) {
	path := t.TempDir()

	store1, err := Open(path)
	if err != nil {
		t.Fatalf("open pebblekv: %v", err)
	}

	order := &types.Order{
		ID:       types.OrderID(1),
		UserID:   types.UserID(1),
		Symbol:   "BTC-USD-PERP",
		Side:     constants.ORDER_SIDE_BUY,
		Category: constants.CATEGORY_LINEAR,
		Type:     constants.ORDER_TYPE_LIMIT,
		Quantity: fixed.NewI(10, 8),
		Price:    fixed.NewI(50000, 8),
	}
	if err := store1.PutOrder(order); err != nil {
		t.Fatalf("put order: %v", err)
	}
	store1.Close()

	store2, err := Open(path)
	if err != nil {
		t.Fatalf("reopen pebblekv: %v", err)
	}

	recovered, err := store2.GetOrder(1)
	if err != nil {
		t.Fatalf("get order after recovery: %v", err)
	}
	if recovered.Quantity.Cmp(fixed.NewI(10, 8)) != 0 {
		t.Errorf("expected quantity 10e-8, got %s", recovered.Quantity.String())
	}

	recovered.Quantity = fixed.NewI(5, 8)
	if err := store2.PutOrder(recovered); err != nil {
		t.Fatalf("update order: %v", err)
	}
	store2.Close()

	store3, err := Open(path)
	if err != nil {
		t.Fatalf("reopen pebblekv again: %v", err)
	}
	defer store3.Close()

	updated, err := store3.GetOrder(1)
	if err != nil {
		t.Fatalf("get order after update: %v", err)
	}
	if updated.Quantity.Cmp(fixed.NewI(5, 8)) != 0 {
		t.Errorf("expected quantity 5e-8 after update, got %s", updated.Quantity.String())
	}
}

func TestPebbleKVDelete(t *testing.T) {
	path := t.TempDir()

	store1, err := Open(path)
	if err != nil {
		t.Fatalf("open pebblekv: %v", err)
	}

	order := &types.Order{
		ID:       types.OrderID(1),
		UserID:   types.UserID(1),
		Symbol:   "BTC-USD-PERP",
		Side:     constants.ORDER_SIDE_BUY,
		Category: constants.CATEGORY_LINEAR,
		Type:     constants.ORDER_TYPE_LIMIT,
		Quantity: fixed.NewI(1, 8),
		Price:    fixed.NewI(50000, 8),
	}
	if err := store1.PutOrder(order); err != nil {
		t.Fatalf("put order: %v", err)
	}
	store1.Close()

	store2, err := Open(path)
	if err != nil {
		t.Fatalf("reopen pebblekv: %v", err)
	}

	if err := store2.DeleteOrder(1); err != nil {
		t.Fatalf("delete order: %v", err)
	}
	store2.Close()

	store3, err := Open(path)
	if err != nil {
		t.Fatalf("reopen pebblekv again: %v", err)
	}
	defer store3.Close()

	_, err = store3.GetOrder(1)
	if err != ErrKeyNotFound {
		t.Errorf("expected ErrKeyNotFound after delete, got %v", err)
	}
}

func TestPebbleKVCheckpoint(t *testing.T) {
	path := t.TempDir()

	store, err := Open(path)
	if err != nil {
		t.Fatalf("open pebblekv: %v", err)
	}
	defer store.Close()

	for i := 0; i < 100; i++ {
		order := &types.Order{
			ID:       types.OrderID(i + 1),
			UserID:   types.UserID(1),
			Symbol:   "BTC-USD-PERP",
			Side:     constants.ORDER_SIDE_BUY,
			Category: constants.CATEGORY_LINEAR,
			Type:     constants.ORDER_TYPE_LIMIT,
			Quantity: fixed.NewI(1, 8),
			Price:    fixed.NewI(50000, 8),
		}
		if err := store.PutOrder(order); err != nil {
			t.Fatalf("put order %d: %v", i, err)
		}
	}

	if err := store.Checkpoint(); err != nil {
		t.Fatalf("checkpoint: %v", err)
	}

	checkpointDir := path + "/checkpoint"
	if _, err := os.Stat(checkpointDir); os.IsNotExist(err) {
		t.Error("checkpoint directory was not created")
	}
}
