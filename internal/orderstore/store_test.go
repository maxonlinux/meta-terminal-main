package orderstore

import (
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func TestStoreAddGetRemove(t *testing.T) {
	store := New()
	order := &types.Order{ID: 1, UserID: 2, Symbol: "BTCUSDT"}
	store.Add(order)

	got := store.Get(2, 1)
	if got == nil {
		t.Fatalf("expected order")
	}

	gotByID := store.GetByID(1)
	if gotByID == nil {
		t.Fatalf("expected order by id")
	}

	store.Remove(2, 1)
	if store.Get(2, 1) != nil {
		t.Fatalf("expected order removed")
	}
	if store.GetByID(1) != nil {
		t.Fatalf("expected order removed by id")
	}
}
