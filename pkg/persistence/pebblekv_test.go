package persistence

import (
	"os"
	"testing"

	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/robaho/fixed"
)

func TestPebbleKVBasic(t *testing.T) {
	path := t.TempDir()
	store, err := Open(path)
	if err != nil {
		t.Fatalf("open pebblekv: %v", err)
	}
	defer store.Close()

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

	if err := store.PutOrder(order); err != nil {
		t.Fatalf("put order: %v", err)
	}

	got, err := store.GetOrder(1)
	if err != nil {
		t.Fatalf("get order: %v", err)
	}
	if got.ID != order.ID {
		t.Errorf("expected order ID %d, got %d", order.ID, got.ID)
	}
}

func TestPebbleKVRange(t *testing.T) {
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

	count := 0
	err = store.RangeOrders(func(order *types.Order) bool {
		count++
		return true
	})
	if err != nil {
		t.Fatalf("range orders: %v", err)
	}
	if count != 100 {
		t.Errorf("expected 100 orders, got %d", count)
	}
}

func TestPebbleKVMeta(t *testing.T) {
	path := t.TempDir()
	store, err := Open(path)
	if err != nil {
		t.Fatalf("open pebblekv: %v", err)
	}
	defer store.Close()

	if err := store.SetMeta("lastSeq", 100); err != nil {
		t.Fatalf("set meta: %v", err)
	}

	lastSeq, err := store.GetMeta("lastSeq")
	if err != nil {
		t.Fatalf("get meta: %v", err)
	}
	if lastSeq != 100 {
		t.Errorf("expected lastSeq 100, got %d", lastSeq)
	}
}

func TestPebbleKVTransaction(t *testing.T) {
	path := t.TempDir()
	store, err := Open(path)
	if err != nil {
		t.Fatalf("open pebblekv: %v", err)
	}
	defer store.Close()

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

	store.Begin()
	if err := store.PutOrder(order); err != nil {
		store.Rollback()
		t.Fatalf("put order: %v", err)
	}
	if err := store.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	got, err := store.GetOrder(1)
	if err != nil {
		t.Fatalf("get order: %v", err)
	}
	if got.ID != order.ID {
		t.Errorf("expected order ID %d, got %d", order.ID, got.ID)
	}
}

func TestPebbleKVTransactionRollback(t *testing.T) {
	path := t.TempDir()
	store, err := Open(path)
	if err != nil {
		t.Fatalf("open pebblekv: %v", err)
	}
	defer store.Close()

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

	store.Begin()
	if err := store.PutOrder(order); err != nil {
		store.Rollback()
		t.Fatalf("put order: %v", err)
	}
	store.Rollback()

	_, err = store.GetOrder(1)
	if err != ErrKeyNotFound {
		t.Errorf("expected ErrKeyNotFound, got %v", err)
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
