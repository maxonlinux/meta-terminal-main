package snapshot

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func TestSnapshot_CreateAndLoad(t *testing.T) {
	tmpDir := t.TempDir()

	snap := New(tmpDir, 100*1024*1024)

	st := state.New()
	st.NextOrderID = 100

	us := st.GetUserState(1)
	us.Balances["USDT"] = &types.UserBalance{
		UserID:    1,
		Asset:     "USDT",
		Available: 1000000,
		Locked:    50000,
		Margin:    100000,
	}

	us.Balances["BTC"] = &types.UserBalance{
		UserID:    1,
		Asset:     "BTC",
		Available: 10,
		Locked:    0,
		Margin:    0,
	}

	us.Positions[1] = &types.Position{
		UserID:      1,
		Symbol:      1,
		Size:        100,
		Side:        0,
		EntryPrice:  50000,
		Leverage:    2,
		RealizedPnl: 5000,
	}

	ss := st.GetSymbolState(1)
	ss.Category = 1
	ss.OrderMap[1] = &types.Order{
		ID:             1,
		UserID:         1,
		Symbol:         1,
		Side:           0,
		Type:           0,
		TIF:            0,
		Status:         0,
		Price:          50000,
		Quantity:       10,
		Filled:         0,
		TriggerPrice:   0,
		StopOrderType:  0,
		ReduceOnly:     false,
		CloseOnTrigger: false,
	}

	err := snap.Create(st, 12345)
	if err != nil {
		t.Fatalf("failed to create snapshot: %v", err)
	}

	loadedSt, offset, err := snap.Load()
	if err != nil {
		t.Fatalf("failed to load snapshot: %v", err)
	}

	if offset != 12345 {
		t.Errorf("expected offset 12345, got %d", offset)
	}

	if loadedSt.NextOrderID != 100 {
		t.Errorf("expected NextOrderID 100, got %d", loadedSt.NextOrderID)
	}

	loadedUs := loadedSt.GetUserState(1)
	bal := loadedUs.Balances["USDT"]
	if bal.Available != 1000000 {
		t.Errorf("expected USDT Available 1000000, got %d", bal.Available)
	}
	if bal.Locked != 50000 {
		t.Errorf("expected USDT Locked 50000, got %d", bal.Locked)
	}
	if bal.Margin != 100000 {
		t.Errorf("expected USDT Margin 100000, got %d", bal.Margin)
	}

	pos := loadedUs.Positions[1]
	if pos.Size != 100 {
		t.Errorf("expected position size 100, got %d", pos.Size)
	}
	if pos.Side != 0 {
		t.Errorf("expected position side 0, got %d", pos.Side)
	}
	if pos.Leverage != 2 {
		t.Errorf("expected position leverage 2, got %d", pos.Leverage)
	}

	loadedSs := loadedSt.GetSymbolState(1)
	if loadedSs.Category != 1 {
		t.Errorf("expected category 1, got %d", loadedSs.Category)
	}
	if len(loadedSs.OrderMap) != 1 {
		t.Errorf("expected 1 order in OrderMap, got %d", len(loadedSs.OrderMap))
	}
}

func TestSnapshot_MultipleSnapshots(t *testing.T) {
	tmpDir := t.TempDir()

	snap := New(tmpDir, 100*1024*1024)

	st1 := state.New()
	err := snap.Create(st1, 1000)
	if err != nil {
		t.Fatalf("failed to create first snapshot: %v", err)
	}

	st2 := state.New()
	st2.NextOrderID = 200
	err = snap.Create(st2, 2000)
	if err != nil {
		t.Fatalf("failed to create second snapshot: %v", err)
	}

	loadedSt, offset, err := snap.Load()
	if err != nil {
		t.Fatalf("failed to load latest snapshot: %v", err)
	}

	if offset != 2000 {
		t.Errorf("expected offset 2000, got %d", offset)
	}

	if loadedSt.NextOrderID != 200 {
		t.Errorf("expected NextOrderID 200, got %d", loadedSt.NextOrderID)
	}
}

func TestSnapshot_NoSnapshot(t *testing.T) {
	tmpDir := t.TempDir()

	snap := New(tmpDir, 100*1024*1024)

	_, _, err := snap.Load()
	if err == nil {
		t.Error("expected error when loading non-existent snapshot")
	}
}

func TestSnapshot_RotateOld(t *testing.T) {
	tmpDir := t.TempDir()

	snap := New(tmpDir, 100*1024*1024)

	for i := 0; i < 10; i++ {
		st := state.New()
		st.NextOrderID = types.OrderID(i)
		err := snap.Create(st, int64(i*1000))
		if err != nil {
			t.Fatalf("failed to create snapshot %d: %v", i, err)
		}
	}

	entries, _ := os.ReadDir(tmpDir)
	snapCount := 0
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".snap" {
			snapCount++
		}
	}

	if snapCount != 5 {
		t.Errorf("expected 5 snapshots after rotation, got %d", snapCount)
	}
}

func TestSnapshot_EmptyState(t *testing.T) {
	tmpDir := t.TempDir()

	snap := New(tmpDir, 100*1024*1024)

	st := state.New()
	err := snap.Create(st, 0)
	if err != nil {
		t.Fatalf("failed to create snapshot: %v", err)
	}

	loadedSt, _, err := snap.Load()
	if err != nil {
		t.Fatalf("failed to load snapshot: %v", err)
	}

	if loadedSt.NextOrderID != 1 {
		t.Errorf("expected NextOrderID 1, got %d", loadedSt.NextOrderID)
	}

	if len(loadedSt.Users) != 0 {
		t.Errorf("expected no users, got %d", len(loadedSt.Users))
	}
}
