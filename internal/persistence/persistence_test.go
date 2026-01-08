package persistence

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/engine"
	"github.com/anomalyco/meta-terminal-go/internal/history"
	"github.com/anomalyco/meta-terminal-go/internal/idgen"
	"github.com/anomalyco/meta-terminal-go/internal/linear"
	"github.com/anomalyco/meta-terminal-go/internal/market"
	"github.com/anomalyco/meta-terminal-go/internal/orderbook"
	"github.com/anomalyco/meta-terminal-go/internal/orderstore"
	"github.com/anomalyco/meta-terminal-go/internal/registry"
	"github.com/anomalyco/meta-terminal-go/internal/snapshot"
	"github.com/anomalyco/meta-terminal-go/internal/spot"
	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/internal/types"
	"github.com/anomalyco/meta-terminal-go/internal/wal"
)

func newEngineForPersistence() *engine.Engine {
	orders := orderstore.New()
	idGen := idgen.NewSnowflake(0)
	books := orderbook.NewStateWithIDGenerator(idGen)
	users := state.NewUsers()
	reg := registry.New()
	triggers := state.NewTriggers()
	var hist history.Reader

	spotClearing := spot.NewClearing(users, reg)
	spotMarket := spot.NewMarket(books, spotClearing)

	linearValidator := linear.NewValidator(users)
	linearClearing := linear.NewClearing(users, reg)
	linearMarket := linear.NewMarket(books, linearValidator, linearClearing)

	markets := map[int8]market.Market{
		spotMarket.GetCategory():   spotMarket,
		linearMarket.GetCategory(): linearMarket,
	}

	eng := engine.New(orders, books, users, reg, triggers, hist, nil, markets, idGen)
	eng.AddInstrument("BTCUSDT", constants.CATEGORY_SPOT, 50000)
	return eng
}

func TestReplayFromSnapshot(t *testing.T) {
	dir := t.TempDir()
	walStore, err := wal.New(filepath.Join(dir, "wal"), 1024*1024, 64*1024)
	if err != nil {
		t.Fatalf("wal init failed: %v", err)
	}
	snapStore, err := snapshot.New(filepath.Join(dir, "snapshots"))
	if err != nil {
		t.Fatalf("snapshot init failed: %v", err)
	}

	eng := newEngineForPersistence()
	mgr := New(eng, walStore, snapStore, 1000, 1024*1024, time.Second)
	_ = eng.SetBalance(1, "USDT", 1_000_000_000)

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_SPOT,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 10,
		Price:    50000,
	}
	res, err := eng.PlaceOrder(input)
	if err != nil {
		t.Fatalf("place failed: %v", err)
	}
	payload := PlaceOrderPayload(res.Order.ID, input)
	if err := mgr.Append(wal.EventPlaceOrder, payload); err != nil {
		t.Fatalf("wal append failed: %v", err)
	}
	if err := mgr.Snapshot(); err != nil {
		t.Fatalf("snapshot failed: %v", err)
	}

	newWal, _ := wal.New(filepath.Join(dir, "wal"), 1024*1024, 64*1024)
	newSnap, _ := snapshot.New(filepath.Join(dir, "snapshots"))
	eng2 := newEngineForPersistence()
	mgr2 := New(eng2, newWal, newSnap, 1000, 1024*1024, time.Second)
	if err := mgr2.Replay(); err != nil {
		t.Fatalf("replay failed: %v", err)
	}

	open := eng2.OpenOrders(1)
	if len(open) != 1 {
		t.Fatalf("expected 1 open order, got %d", len(open))
	}
}
