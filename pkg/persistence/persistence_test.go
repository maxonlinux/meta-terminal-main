package persistence

import (
	"testing"

	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/robaho/fixed"
)

func TestStoreRecovery(t *testing.T) {
	path := t.TempDir()

	store1, err := Open(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
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
		if err := store1.SaveOrder(order); err != nil {
			t.Fatalf("save order %d: %v", i, err)
		}
	}

	if err := store1.Close(); err != nil {
		t.Fatalf("close store1: %v", err)
	}

	store2, err := Open(path)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	defer store2.Close()

	count := 0
	err = store2.LoadOrders(func(order *types.Order) bool {
		count++
		if order.ID == types.OrderID(0) {
			t.Error("invalid order ID 0")
		}
		return true
	})
	if err != nil {
		t.Fatalf("load orders: %v", err)
	}
	if count != 100 {
		t.Errorf("expected 100 orders after recovery, got %d", count)
	}
}

func TestStoreRecoveryUpdate(t *testing.T) {
	path := t.TempDir()

	store1, err := Open(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
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
	if err := store1.SaveOrder(order); err != nil {
		t.Fatalf("save order: %v", err)
	}
	store1.Close()

	store2, err := Open(path)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}

	recovered, err := store2.GetOrder(1)
	if err != nil {
		t.Fatalf("get order after recovery: %v", err)
	}
	if recovered.Quantity.Cmp(fixed.NewI(10, 8)) != 0 {
		t.Errorf("expected quantity 10e-8, got %s", recovered.Quantity.String())
	}

	recovered.Quantity = fixed.NewI(5, 8)
	if err := store2.SaveOrder(recovered); err != nil {
		t.Fatalf("update order: %v", err)
	}
	store2.Close()

	store3, err := Open(path)
	if err != nil {
		t.Fatalf("reopen store again: %v", err)
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

func TestStoreKeyValue(t *testing.T) {
	path := t.TempDir()

	store, err := Open(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	key := []byte("test:key")
	value := []byte("test_value")

	if err := store.Set(key, value); err != nil {
		t.Fatalf("set key: %v", err)
	}

	got, err := store.Get(key)
	if err != nil {
		t.Fatalf("get key: %v", err)
	}
	if string(got) != string(value) {
		t.Errorf("expected %s, got %s", value, got)
	}

	_, err = store.Get([]byte("nonexistent"))
	if err != ErrKeyNotFound {
		t.Errorf("expected ErrKeyNotFound, got %v", err)
	}
}
