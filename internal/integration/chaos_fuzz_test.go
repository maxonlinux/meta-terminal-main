package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/anomalyco/meta-terminal-go/internal/api"
)

type ChaosTestServer struct {
	*TestServer
}

func NewChaosTestServer(t *testing.T) *ChaosTestServer {
	ts := NewTestServer(t)
	return &ChaosTestServer{ts}
}

func (ts *ChaosTestServer) GetOrder(orderID int64) *api.OrderResponse {
	url := fmt.Sprintf("%s/api/v1/orders/%d?userId=1", ts.URL(), orderID)
	resp, err := http.Get(url)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	var orderResp api.OrderResponse
	json.NewDecoder(resp.Body).Decode(&orderResp)
	return &orderResp
}

func TestChaos_RapidOrders(t *testing.T) {
	ts := NewChaosTestServer(t)
	defer ts.Close()

	ts.SeedBalance(1, 10000000)
	ts.SeedBalance(2, 10000000)
	ts.SeedBalance(3, 10000000)

	for i := 0; i < 100; i++ {
		userID := (i % 3) + 1
		side := i % 2
		quantity := int64(1 + (i % 50))
		price := int64(10000 + (i % 10000))
		orderType := i % 2
		ts.PlaceOrder(userID, 1, side, orderType, 0, quantity, price)
	}

	ts.PlaceOrder(1, 1, 1, 1, 0, 100, 0)
	ts.PlaceOrder(2, 1, 0, 1, 0, 100, 0)
	ts.PlaceOrder(3, 1, 1, 1, 0, 100, 0)
	ts.PlaceOrder(1, 1, 0, 1, 0, 100, 0)

	pos := ts.GetPosition(1, 1)
	if pos == nil || pos.Size == 0 {
		t.Errorf("expected position after chaos orders")
	}
}

func TestChaos_CancelRace(t *testing.T) {
	ts := NewChaosTestServer(t)
	defer ts.Close()

	ts.SeedBalance(1, 1000000)

	var orderIDs []int64
	for i := 0; i < 50; i++ {
		resp := ts.PlaceOrder(1, 1, 0, 0, 0, 10, int64(40000+i%1000))
		orderIDs = append(orderIDs, resp.OrderID)
	}

	for i, orderID := range orderIDs {
		if i%2 == 0 {
			ts.CancelOrder(orderID)
		}
	}

	for _, orderID := range orderIDs {
		if orderID%2 == 0 {
			resp := ts.GetOrder(orderID)
			if resp != nil && resp.Status != 3 {
				t.Errorf("order %d should be cancelled, got status %d", orderID, resp.Status)
			}
		}
	}
}

func TestChaos_EditLeverageDuringTrading(t *testing.T) {
	ts := NewChaosTestServer(t)
	defer ts.Close()

	ts.SeedBalance(1, 10000000)
	ts.SeedBalance(2, 10000000)

	go func() {
		for i := 0; i < 20; i++ {
			ts.EditLeverage(1, 1, 2+(i%10))
			time.Sleep(10 * time.Millisecond)
		}
	}()

	for i := 0; i < 50; i++ {
		ts.PlaceOrder(1, 1, 0, 0, 0, 10, 50000)
		time.Sleep(5 * time.Millisecond)
	}

	ts.PlaceOrder(2, 1, 1, 1, 0, 100, 50000)
	pos := ts.GetPosition(1, 1)
	if pos == nil {
		t.Errorf("expected position to exist")
	}
}

func TestChaos_SnapshotDuringTrading(t *testing.T) {
	ts := NewChaosTestServer(t)
	defer ts.Close()

	ts.SeedBalance(1, 5000000)
	ts.SeedBalance(2, 5000000)

	for i := 0; i < 100; i++ {
		userID := (i % 2) + 1
		side := i % 2
		ts.PlaceOrder(userID, 1, side, 0, 0, 10, 50000)
	}

	offset := ts.w.Offset()
	_ = ts.snap.Create(ts.s, offset)
	t.Logf("snapshot offset: %d", offset)
}

func TestChaos_CrashRecovery(t *testing.T) {
	tmpDir := t.TempDir()

	ts := NewTestServerWithDir(t, tmpDir)
	defer ts.Close()

	ts.SeedBalance(1, 1000000)
	ts.SeedBalance(2, 1000000)

	ts.PlaceOrder(1, 1, 0, 0, 0, 10, 50000)

	err := ts.snap.Create(ts.s, ts.w.Offset())
	if err != nil {
		t.Fatalf("create snapshot failed: %v", err)
	}

	ts.Close()

	ts2 := NewTestServerWithDir(t, tmpDir)
	defer ts2.Close()

	state2, _, err := ts2.snap.Load()
	if err != nil {
		t.Fatalf("snapshot load failed: %v", err)
	}
	_ = state2
}

func TestChaos_ManyUsers(t *testing.T) {
	ts := NewChaosTestServer(t)
	defer ts.Close()

	ts.SeedBalance(1, 1000000)
	ts.SeedBalance(2, 1000000)

	ts.PlaceOrder(1, 1, 0, 0, 0, 10, 50000)
	ts.PlaceOrder(2, 1, 1, 0, 0, 5, 50000)

	pos := ts.GetPosition(1, 1)
	if pos != nil {
		t.Logf("position size: %d", pos.Size)
	}
}

func TestChaos_WALEventsDuringSnapshot(t *testing.T) {
	ts := NewChaosTestServer(t)
	defer ts.Close()

	ts.SeedBalance(1, 10000000)
	ts.SeedBalance(2, 1000000)

	ts.PlaceOrder(1, 1, 0, 0, 0, 10, 50000)
	ts.PlaceOrder(2, 1, 1, 0, 0, 5, 50000)

	eventsBefore := ts.w.EventCount()
	if eventsBefore == 0 {
		t.Errorf("expected some events before snapshot")
	}

	_ = ts.snap.Create(ts.s, ts.w.Offset())
}

func TestFuzz_OrderInputs(t *testing.T) {
	ts := NewChaosTestServer(t)
	defer ts.Close()

	ts.SeedBalance(1, 10000000)
	ts.SeedBalance(2, 1000000)

	fuzzValues := []struct {
		quantity int64
		price    int64
	}{
		{1, 1},
		{1, 100000000},
		{1000000, 1},
		{1, 0},
		{0, 50000},
	}

	for _, fv := range fuzzValues {
		ts.PlaceOrder(1, 1, 0, 0, 0, fv.quantity, fv.price)
	}

	ts.PlaceOrder(2, 1, 1, 1, 0, 1, 50000)
}

func TestFuzz_ExtremeLeverage(t *testing.T) {
	ts := NewChaosTestServer(t)
	defer ts.Close()

	ts.SeedBalance(1, 1000000)

	testCases := []int{1, 2, 10, 50, 100, 101, 0, -1}
	for _, lev := range testCases {
		err := ts.EditLeverage(1, 1, lev)
		if lev >= 1 && lev <= 100 {
			if err != nil {
				t.Errorf("leverage %d should succeed, got: %v", lev, err)
			}
		}
	}
}

func TestFuzz_Sequence(t *testing.T) {
	ts := NewChaosTestServer(t)
	defer ts.Close()

	ts.SeedBalance(1, 10000000)
	ts.SeedBalance(2, 10000000)

	sequence := []struct {
		userID    int
		side      int
		orderType int
		quantity  int64
		price     int64
	}{
		{1, 0, 0, 10, 50000},
		{2, 1, 0, 5, 50000},
		{1, 1, 1, 3, 0},
		{2, 0, 0, 2, 49000},
		{1, 0, 3, 1, 49500},
	}

	for _, s := range sequence {
		resp := ts.PlaceOrder(s.userID, 1, s.side, s.orderType, 0, s.quantity, s.price)
		if resp.Status < 0 || resp.Status > 7 {
			t.Errorf("invalid status: %d", resp.Status)
		}
	}
}

func TestFuzz_MixedCategories(t *testing.T) {
	ts := NewChaosTestServer(t)
	defer ts.Close()

	ts.SeedBalance(1, 1000000)
	ts.SeedBalance(2, 1000000)

	ts.PlaceOrder(1, 1, 0, 0, 0, 10, 50000)
	ts.PlaceOrder(2, 1, 1, 0, 0, 5, 50000)

	ts.PlaceOrder(1, 1, 1, 1, 0, 5, 0)

	pos1 := ts.GetPosition(1, 1)
	if pos1 != nil {
		t.Logf("pos1 size: %d", pos1.Size)
	}
}

func TestFuzz_ReduceOnly(t *testing.T) {
	ts := NewChaosTestServer(t)
	defer ts.Close()

	ts.SeedBalance(1, 1000000)
	ts.SeedBalance(2, 1000000)

	ts.PlaceOrder(1, 1, 0, 0, 0, 10, 50000)
	ts.PlaceOrder(2, 1, 1, 0, 0, 5, 50000)

	ts.PlaceOrder(1, 1, 1, 0, 0, 3, 51000)
}

func TestFuzz_TriggerOrders(t *testing.T) {
	ts := NewChaosTestServer(t)
	defer ts.Close()

	ts.SeedBalance(1, 1000000)
	ts.SeedBalance(2, 1000000)

	ts.PlaceOrder(1, 1, 1, 0, 0, 10, 45000)
	ts.PlaceOrder(2, 1, 0, 0, 0, 10, 50000)

	resp := ts.PlaceOrder(1, 1, 0, 0, 0, 5, 44000, WithTriggerPrice(45000))
	if resp.Status != 5 {
		t.Errorf("expected UNTRIGGERED status, got %d", resp.Status)
	}
}

func TestFuzz_IOCFOK(t *testing.T) {
	ts := NewChaosTestServer(t)
	defer ts.Close()

	ts.SeedBalance(1, 1000000)
	ts.SeedBalance(2, 1000000)

	ts.PlaceOrder(2, 1, 1, 0, 0, 5, 50000)

	resp := ts.PlaceOrder(1, 1, 0, 1, 1, 10, 50000)
	if resp.Filled != 5 {
		t.Errorf("IOC should fill available liquidity, got %d", resp.Filled)
	}
}

func TestFuzz_PostOnly(t *testing.T) {
	ts := NewChaosTestServer(t)
	defer ts.Close()

	ts.SeedBalance(1, 1000000)
	ts.SeedBalance(2, 1000000)

	ts.PlaceOrder(2, 1, 1, 0, 0, 10, 50000)

	resp := ts.PlaceOrder(1, 1, 0, 0, 3, 10, 50000)
	if resp.Status != 3 {
		t.Errorf("POST_ONLY should be rejected when crossing spread, got status %d", resp.Status)
	}

	resp2 := ts.PlaceOrder(1, 1, 0, 0, 3, 10, 49999)
	if resp2.Status != 0 {
		t.Errorf("POST_ONLY should be accepted when not crossing, got status %d", resp2.Status)
	}
}

func TestFuzz_LeverageBoundaries(t *testing.T) {
	ts := NewChaosTestServer(t)
	defer ts.Close()

	ts.SeedBalance(1, 1000000)

	boundaries := []int{0, 1, 2, 50, 99, 100, 101, 255}
	for _, lev := range boundaries {
		err := ts.EditLeverage(1, 1, lev)
		if lev >= 1 && lev <= 100 {
			if err != nil {
				t.Errorf("leverage %d should be valid", lev)
			}
		}
	}
}

func TestFuzz_PriceBoundaries(t *testing.T) {
	ts := NewChaosTestServer(t)
	defer ts.Close()

	ts.SeedBalance(1, 10000000)
	ts.SeedBalance(2, 1000000)

	prices := []int64{0, 1, 1000000000000}
	for _, price := range prices {
		ts.PlaceOrder(1, 1, 0, 0, 0, 1, price)
	}

	ts.PlaceOrder(2, 1, 1, 1, 0, 1, 50000)
}

func TestFuzz_QuantityBoundaries(t *testing.T) {
	ts := NewChaosTestServer(t)
	defer ts.Close()

	ts.SeedBalance(1, 1000000000)
	ts.SeedBalance(2, 1000000)

	quantities := []int64{0, 1}
	for _, qty := range quantities {
		ts.PlaceOrder(1, 1, 0, 0, 0, qty, 50000)
	}

	ts.PlaceOrder(2, 1, 1, 1, 0, 1, 50000)
}

func TestChaos_ConcurrentOrderAndCancel(t *testing.T) {
	ts := NewChaosTestServer(t)
	defer ts.Close()

	ts.SeedBalance(1, 1000000)

	resp := ts.PlaceOrder(1, 1, 0, 0, 0, 10, 50000)
	ts.CancelOrder(resp.OrderID)

	resp2 := ts.GetOrder(resp.OrderID)
	if resp2 != nil && resp2.Status != 3 {
		t.Errorf("order should be cancelled")
	}
}

func TestChaos_SnapshotPersistence(t *testing.T) {
	tmpDir := t.TempDir()

	ts := NewTestServerWithDir(t, tmpDir)
	defer ts.Close()

	ts.SeedBalance(1, 1000000)
	ts.SeedBalance(2, 1000000)

	ts.PlaceOrder(1, 1, 0, 0, 0, 10, 50000)
	ts.PlaceOrder(2, 1, 1, 0, 0, 5, 50000)

	offsetBefore := ts.w.Offset()
	err := ts.snap.Create(ts.s, offsetBefore)
	if err != nil {
		t.Fatalf("create snapshot failed: %v", err)
	}

	state1, _, _ := ts.snap.Load()
	us1 := state1.GetUserState(1)
	pos1 := us1.Positions[1]

	ts.PlaceOrder(2, 1, 1, 1, 0, 3, 0)

	_ = ts.snap.Create(ts.s, ts.w.Offset())
	state2, _, _ := ts.snap.Load()
	us2 := state2.GetUserState(1)
	pos2 := us2.Positions[1]

	if pos1 == nil {
		t.Errorf("pos1 should not be nil")
	}
	if pos2 == nil {
		t.Errorf("pos2 should not be nil")
	}
}

func TestChaos_ManySnapshots(t *testing.T) {
	ts := NewChaosTestServer(t)
	defer ts.Close()

	ts.SeedBalance(1, 1000000)
	ts.SeedBalance(2, 1000000)

	for i := 0; i < 10; i++ {
		ts.PlaceOrder(1, 1, 0, 0, 0, 1, int64(50000+i%1000))
		ts.PlaceOrder(2, 1, 1, 0, 0, 1, int64(50000+i%1000))
		if i%2 == 0 {
			_ = ts.snap.Create(ts.s, ts.w.Offset())
		}
	}

	state, _, err := ts.snap.Load()
	if err != nil {
		t.Fatalf("load snapshot failed: %v", err)
	}
	_ = state
}

func TestFuzz_EmptyOrders(t *testing.T) {
	ts := NewChaosTestServer(t)
	defer ts.Close()

	ts.SeedBalance(1, 1000000)
	ts.SeedBalance(2, 1000000)

	ts.PlaceOrder(1, 1, 0, 0, 0, 0, 50000)
	ts.PlaceOrder(2, 1, 1, 0, 0, 0, 50000)

	ts.PlaceOrder(1, 1, 0, 0, 0, 10, 0)
	ts.PlaceOrder(2, 1, 1, 1, 0, 10, 0)
}

func TestChaos_MixedOrderTypes(t *testing.T) {
	ts := NewChaosTestServer(t)
	defer ts.Close()

	ts.SeedBalance(1, 5000000)
	ts.SeedBalance(2, 5000000)

	ts.PlaceOrder(2, 1, 1, 0, 0, 100, 50000)
	ts.PlaceOrder(2, 1, 0, 0, 0, 100, 49000)

	resp := ts.PlaceOrder(1, 1, 0, 1, 1, 50, 0)
	if resp.Filled != 50 {
		t.Errorf("FOK should fill or cancel completely, got %d", resp.Filled)
	}

	resp2 := ts.PlaceOrder(1, 1, 1, 1, 1, 100, 0)
	if resp2.Filled != 100 {
		t.Errorf("FOK should fill available liquidity, got %d", resp2.Filled)
	}
}
