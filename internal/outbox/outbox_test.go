package outbox

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/anomalyco/meta-terminal-go/internal/storage"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "engine.db")
	db, err := storage.OpenSQLite(path)
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := storage.EnsureSchema(db); err != nil {
		_ = db.Close()
		t.Fatalf("schema init failed: %v", err)
	}
	return db
}

func TestOutboxAppendAndFlush(t *testing.T) {
	db := openTestDB(t)
	defer func() { _ = db.Close() }()

	box := NewSQLite(db, 10, time.Hour)

	trade := &types.Trade{
		ID:           1,
		Symbol:       "BTCUSDT",
		Category:     0,
		TakerID:      10,
		MakerID:      11,
		TakerOrderID: 100,
		MakerOrderID: 101,
		Price:        50000,
		Quantity:     2,
		ExecutedAt:   123,
	}
	if err := box.AppendTrade(trade); err != nil {
		t.Fatalf("append trade failed: %v", err)
	}
	if err := box.Flush(); err != nil {
		t.Fatalf("flush failed: %v", err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(1) FROM trades_history`).Scan(&count); err != nil {
		t.Fatalf("count trades failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 trade, got %d", count)
	}
}

func TestOutboxOrderInsert(t *testing.T) {
	db := openTestDB(t)
	defer func() { _ = db.Close() }()

	box := NewSQLite(db, 1, time.Hour)

	order := &types.Order{
		ID:           7,
		UserID:       99,
		Symbol:       "BTCUSDT",
		Category:     0,
		Side:         0,
		Type:         0,
		TIF:          0,
		Status:       2,
		Price:        50000,
		Quantity:     3,
		Filled:       3,
		TriggerPrice: 0,
		ReduceOnly:   false,
		CreatedAt:    1,
		UpdatedAt:    2,
	}
	if err := box.AppendOrder(order); err != nil {
		t.Fatalf("append order failed: %v", err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(1) FROM orders_history`).Scan(&count); err != nil {
		t.Fatalf("count orders failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 order, got %d", count)
	}
}
