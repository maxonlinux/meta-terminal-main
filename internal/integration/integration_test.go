package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/anomalyco/meta-terminal-go/internal/api"
	"github.com/anomalyco/meta-terminal-go/internal/config"
	"github.com/anomalyco/meta-terminal-go/internal/engine"
	"github.com/anomalyco/meta-terminal-go/internal/snapshot"
	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/internal/types"
	"github.com/anomalyco/meta-terminal-go/internal/wal"
)

type TestServer struct {
	t      *testing.T
	cfg    *config.Config
	s      *state.State
	w      *wal.WAL
	snap   *snapshot.Snapshot
	e      *engine.Engine
	srv    *api.Server
	server *httptest.Server
}

func NewTestServer(t *testing.T) *TestServer {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		ServerHost: "127.0.0.1",
		ServerPort: 8080,
	}

	st := state.New()

	w, err := wal.New(tmpDir+"/wal", 64)
	if err != nil {
		t.Fatalf("failed to create WAL: %v", err)
	}

	snap := snapshot.New(tmpDir+"/snapshots", 100*1024*1024)

	e := engine.New(w, st)

	s := api.NewServer(cfg, e)

	ctx := context.Background()
	go s.Start(ctx)

	time.Sleep(100 * time.Millisecond)

	return &TestServer{
		t:      t,
		cfg:    cfg,
		s:      st,
		w:      w,
		snap:   snap,
		e:      e,
		srv:    s,
		server: httptest.NewServer(s.Router()),
	}
}

func (ts *TestServer) Close() {
	ts.server.Close()
	ts.w.Close()
}

func (ts *TestServer) SeedBalance(userID int, available int64) {
	us := ts.s.GetUserState(types.UserID(userID))
	us.Balances["USDT"] = &types.UserBalance{
		UserID:    types.UserID(userID),
		Asset:     "USDT",
		Available: available,
		Locked:    0,
		Margin:    0,
		Version:   0,
	}
}

func (ts *TestServer) URL() string {
	return ts.server.URL
}

func (ts *TestServer) PlaceOrder(userID, symbol, side, orderType, tif int, quantity, price int64, opts ...OrderOption) *PlaceOrderResponse {
	req := map[string]interface{}{
		"userId":   userID,
		"symbol":   symbol,
		"category": 1,
		"side":     side,
		"type":     orderType,
		"tif":      tif,
		"quantity": quantity,
		"price":    price,
	}

	for _, opt := range opts {
		opt(req)
	}

	body, _ := json.Marshal(req)
	resp, err := http.Post(ts.URL()+"/api/v1/orders", "application/json", strings.NewReader(string(body)))
	if err != nil {
		ts.t.Fatalf("failed to place order: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]string
		json.NewDecoder(resp.Body).Decode(&errResp)
		ts.t.Fatalf("place order failed: %d %v", resp.StatusCode, errResp)
	}

	var result PlaceOrderResponse
	json.NewDecoder(resp.Body).Decode(&result)
	return &result
}

type OrderOption func(map[string]interface{})

func WithTriggerPrice(price int64) OrderOption {
	return func(m map[string]interface{}) {
		m["triggerPrice"] = price
	}
}

func WithReduceOnly() OrderOption {
	return func(m map[string]interface{}) {
		m["reduceOnly"] = true
	}
}

func WithCloseOnTrigger() OrderOption {
	return func(m map[string]interface{}) {
		m["closeOnTrigger"] = true
	}
}

type PlaceOrderResponse struct {
	OrderID   int64 `json:"orderId"`
	Status    int8  `json:"status"`
	Filled    int64 `json:"filled"`
	Remaining int64 `json:"remaining"`
}

func (ts *TestServer) CancelOrder(orderID int64) {
	url := ts.URL() + "/api/v1/orders/" + strconv.FormatInt(orderID, 10)
	req, _ := http.NewRequest("DELETE", url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		ts.t.Fatalf("failed to cancel order: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		ts.t.Fatalf("cancel order failed: %d", resp.StatusCode)
	}
}

func (ts *TestServer) GetPosition(userID, symbol int) *types.Position {
	url := fmt.Sprintf("%s/api/v1/1/%d/positions?userId=%d", ts.URL(), symbol, userID)
	resp, err := http.Get(url)
	if err != nil {
		ts.t.Fatalf("failed to get position: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	var posResp api.PositionResponse
	json.NewDecoder(resp.Body).Decode(&posResp)

	// If API returns empty response (no position at all), return nil
	if posResp.Size == 0 && posResp.Leverage == 0 {
		return nil
	}

	return &types.Position{
		UserID:      types.UserID(userID),
		Symbol:      types.SymbolID(symbol),
		Size:        types.Quantity(posResp.Size),
		Side:        posResp.Side,
		EntryPrice:  types.Price(posResp.EntryPrice),
		Leverage:    posResp.Leverage,
		RealizedPnl: posResp.RealizedPnl,
	}
}

func (ts *TestServer) GetBalances(userID int) []*types.UserBalance {
	url := fmt.Sprintf("%s/api/v1/balances?userId=%d", ts.URL(), userID)
	resp, err := http.Get(url)
	if err != nil {
		ts.t.Fatalf("failed to get balances: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		ts.t.Fatalf("get balances failed: %d", resp.StatusCode)
	}

	var balances []*types.UserBalance
	json.NewDecoder(resp.Body).Decode(&balances)
	return balances
}

func (ts *TestServer) EditLeverage(userID, symbol, leverage int) error {
	body, _ := json.Marshal(map[string]interface{}{
		"userId":   userID,
		"symbol":   symbol,
		"leverage": leverage,
	})

	url := fmt.Sprintf("%s/api/v1/1/%d/position/leverage", ts.URL(), symbol)
	req, _ := http.NewRequest("PATCH", url, strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]string
		json.NewDecoder(resp.Body).Decode(&errResp)
		return &APIError{Message: errResp["error"]}
	}
	return nil
}

type APIError struct {
	Message string
}

func (e *APIError) Error() string {
	return e.Message
}

func (ts *TestServer) ClosePosition(userID, symbol int) error {
	body, _ := json.Marshal(map[string]interface{}{
		"userId": userID,
		"symbol": symbol,
	})

	url := fmt.Sprintf("%s/api/v1/1/%d/position", ts.URL(), symbol)
	req, _ := http.NewRequest("DELETE", url, strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]string
		json.NewDecoder(resp.Body).Decode(&errResp)
		return &APIError{Message: errResp["error"]}
	}
	return nil
}

func TestIntegration_FullSpotFlow(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	resp := ts.PlaceOrder(1, 1, 0, 0, 0, 10, 50000)
	if resp.OrderID != 1 {
		t.Errorf("expected order ID 1, got %d", resp.OrderID)
	}
	if resp.Status != 0 {
		t.Errorf("expected status NEW (0), got %d", resp.Status)
	}
}

func TestIntegration_LinearOpenPosition(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	ts.SeedBalance(1, 1000000)
	ts.SeedBalance(2, 1000000)

	resp1 := ts.PlaceOrder(1, 1, 0, 0, 0, 10, 50000)
	resp2 := ts.PlaceOrder(2, 1, 1, 1, 0, 10, 0)

	if resp1.OrderID != 1 {
		t.Errorf("expected order ID 1, got %d", resp1.OrderID)
	}
	if resp1.Status != 0 {
		t.Errorf("expected order 1 NEW (0), got %d", resp1.Status)
	}
	if resp2.Status != 2 {
		t.Errorf("expected order 2 FILLED (2), got %d", resp2.Status)
	}

	pos := ts.GetPosition(1, 1)
	if pos.Size != 10 {
		t.Errorf("expected position size 10, got %d", pos.Size)
	}
	if pos.Leverage != 2 {
		t.Errorf("expected leverage 2, got %d", pos.Leverage)
	}
}

func TestIntegration_TwoUsersTrade(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	ts.SeedBalance(1, 1000000)
	ts.SeedBalance(2, 1000000)

	resp1 := ts.PlaceOrder(1, 1, 0, 0, 0, 10, 50000)
	resp2 := ts.PlaceOrder(2, 1, 1, 1, 0, 10, 0)

	if resp1.Status != 0 {
		t.Errorf("expected order 1 NEW (0), got %d", resp1.Status)
	}
	if resp2.Status != 2 {
		t.Errorf("expected order 2 FILLED (2), got %d", resp2.Status)
	}

	pos1 := ts.GetPosition(1, 1)
	if pos1.Size != 10 {
		t.Errorf("user 1: expected size 10, got %d", pos1.Size)
	}

	pos2 := ts.GetPosition(2, 1)
	if pos2.Size != 10 || pos2.Side != 1 {
		t.Errorf("user 2: expected size 10, side SHORT (1), got %d, %d", pos2.Size, pos2.Side)
	}
}

func TestIntegration_EditLeverage(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	ts.SeedBalance(1, 1000000)

	err := ts.EditLeverage(1, 1, 10)
	if err != nil {
		t.Errorf("EditLeverage failed: %v", err)
	}

	pos := ts.GetPosition(1, 1)
	if pos == nil {
		t.Fatalf("position is nil - EditLeverage should create empty position")
	}
	if pos.Leverage != 10 {
		t.Errorf("expected leverage 10, got %d", pos.Leverage)
	}
	if pos.Size != 0 {
		t.Errorf("expected size 0 (empty position), got %d", pos.Size)
	}
}

func TestIntegration_ClosePosition(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	ts.SeedBalance(1, 1000000)
	ts.SeedBalance(2, 1000000)

	// Create a position first
	ts.PlaceOrder(2, 1, 1, 0, 0, 10, 50000) // User 2 LIMIT SELL
	ts.PlaceOrder(1, 1, 0, 1, 0, 10, 0)     // User 1 MARKET BUY to open position

	pos := ts.GetPosition(1, 1)
	if pos == nil {
		t.Fatalf("position is nil")
	}
	if pos.Size != 10 {
		t.Errorf("expected size 10, got %d", pos.Size)
	}

	err := ts.ClosePosition(1, 1)
	if err != nil {
		t.Errorf("ClosePosition failed: %v", err)
	}

	pos = ts.GetPosition(1, 1)
	if pos != nil && pos.Size != 0 {
		t.Errorf("expected position to be closed, got size %d", pos.Size)
	}
}

func TestIntegration_CancelOrder(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	ts.SeedBalance(1, 1000000)

	resp := ts.PlaceOrder(1, 1, 0, 0, 0, 10, 50000)
	if resp.OrderID != 1 {
		t.Errorf("expected order ID 1, got %d", resp.OrderID)
	}

	ts.CancelOrder(1)
}

func TestIntegration_MultipleOrdersSamePrice(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	ts.SeedBalance(1, 1000000)
	ts.SeedBalance(2, 1000000)
	ts.SeedBalance(3, 1000000)

	ts.PlaceOrder(1, 1, 0, 0, 0, 5, 50000)
	ts.PlaceOrder(2, 1, 0, 0, 0, 5, 50000)

	resp := ts.PlaceOrder(3, 1, 1, 1, 0, 8, 0)

	if resp.Filled != 8 {
		t.Errorf("expected filled 8, got %d", resp.Filled)
	}

	pos := ts.GetPosition(3, 1)
	if pos.Size != 8 || pos.Side != 1 {
		t.Errorf("expected size 8, side SHORT (1), got %d, %d", pos.Size, pos.Side)
	}
}

func TestIntegration_PartialFill(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	ts.SeedBalance(1, 1000000)
	ts.SeedBalance(2, 1000000)

	ts.PlaceOrder(1, 1, 0, 0, 0, 5, 50000)

	resp := ts.PlaceOrder(2, 1, 1, 1, 1, 10, 0)

	if resp.Filled != 5 {
		t.Errorf("expected filled 5, got %d", resp.Filled)
	}
	if resp.Status != 4 {
		t.Errorf("expected status PARTIAL_CANCELED (4), got %d", resp.Status)
	}
}

func TestIntegration_ReduceOnly(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	ts.SeedBalance(1, 1000000)
	ts.SeedBalance(2, 1000000)

	// User 2 places LIMIT SELL 10 @ 50000 to provide liquidity
	ts.PlaceOrder(2, 1, 1, 0, 0, 10, 50000)

	// User 1 places MARKET BUY 10 to open LONG position
	ts.PlaceOrder(1, 1, 0, 1, 0, 10, 0)

	// User 1 places MARKET SELL 5 with reduceOnly
	resp := ts.PlaceOrder(1, 1, 1, 1, 0, 5, 0, WithReduceOnly())

	if resp.Status != 2 {
		t.Errorf("expected FILLED (2), got %d", resp.Status)
	}

	pos := ts.GetPosition(1, 1)
	if pos.Size != 5 {
		t.Errorf("expected size 5, got %d", pos.Size)
	}
}

func TestIntegration_BalanceTracking(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	ts.SeedBalance(1, 1000000)

	// Place LIMIT order - this should lock the balance
	ts.PlaceOrder(1, 1, 0, 0, 0, 10, 50000) // LIMIT BUY 10 @ 50000

	balances := ts.GetBalances(1)
	marginRequired := int64(10) * 50000 / 2
	for _, bal := range balances {
		if bal.Asset == "USDT" {
			if bal.Locked != marginRequired {
				t.Errorf("expected locked %d, got %d", marginRequired, bal.Locked)
			}
		}
	}
}

func TestIntegration_IOCOrder(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	ts.SeedBalance(1, 1000000)
	ts.SeedBalance(2, 1000000)

	ts.PlaceOrder(1, 1, 0, 0, 0, 5, 50000)

	resp := ts.PlaceOrder(2, 1, 1, 1, 1, 100, 0)

	if resp.Status != 4 {
		t.Errorf("expected PARTIAL_CANCELED (4), got %d", resp.Status)
	}
	if resp.Filled != 5 {
		t.Errorf("expected filled 5, got %d", resp.Filled)
	}
}

func TestIntegration_SnapshotAndRecovery(t *testing.T) {
	ts := NewTestServer(t)

	ts.SeedBalance(1, 1000000)
	ts.SeedBalance(2, 1000000)

	// Create a position first - need liquidity from user 2
	ts.PlaceOrder(2, 1, 1, 0, 0, 10, 50000) // User 2 LIMIT SELL
	ts.PlaceOrder(1, 1, 0, 1, 0, 10, 0)     // User 1 MARKET BUY to open position

	err := ts.snap.Create(ts.s, ts.w.Offset())
	if err != nil {
		ts.t.Fatalf("failed to create snapshot: %v", err)
	}

	state2, offset, err := ts.snap.Load()
	if err != nil {
		ts.t.Fatalf("failed to load snapshot: %v", err)
	}

	if offset != ts.w.Offset() {
		ts.t.Errorf("expected offset %d, got %d", ts.w.Offset(), offset)
	}

	us := state2.GetUserState(1)
	pos := us.Positions[1]
	if pos == nil || pos.Size != 10 {
		ts.t.Errorf("expected position size 10 after recovery, got %v", pos)
	}

	ts.Close()
}

func TestIntegration_PositionSideUpdates(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	ts.SeedBalance(1, 1000000)
	ts.SeedBalance(2, 1000000)

	// User 2 provides both buy and sell liquidity
	ts.PlaceOrder(2, 1, 1, 0, 0, 20, 50000) // LIMIT SELL 20 @ 50000
	ts.PlaceOrder(2, 1, 0, 0, 0, 20, 49000) // LIMIT BUY 20 @ 49000

	// User 1 opens SHORT position by selling to LIMIT BUY @ 49000
	ts.PlaceOrder(1, 1, 1, 1, 0, 10, 0) // MARKET SELL 10
	pos := ts.GetPosition(1, 1)
	t.Logf("After SELL 10: pos=%v", pos)
	if pos == nil || pos.Side != 1 {
		t.Errorf("expected SHORT position (side 1), got %v", pos)
	}
	if pos.Size != 10 {
		t.Errorf("expected size 10, got %d", pos.Size)
	}

	// User 1 flips to LONG by buying from LIMIT SELL @ 50000
	// BUY 15: closes SHORT 10 (10 sold @ 49000, bought @ 50000 = -10000 PnL)
	//         opens LONG 5  (5 bought @ 50000)
	ts.PlaceOrder(1, 1, 0, 1, 0, 15, 0) // MARKET BUY 15
	pos = ts.GetPosition(1, 1)
	t.Logf("After BUY 15: pos=%v", pos)
	if pos == nil || pos.Side != 0 {
		t.Errorf("expected LONG position (side 0), got %v", pos)
	}
	if pos.Size != 5 {
		t.Errorf("expected size 5 (15 - 10 = 5), got %d", pos.Size)
	}
}

func TestIntegration_LeverageIncreaseReleasesMargin(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	ts.SeedBalance(1, 1000000)
	ts.SeedBalance(2, 1000000)

	// Create position first
	ts.PlaceOrder(2, 1, 1, 0, 0, 10, 50000) // User 2 LIMIT SELL
	ts.PlaceOrder(1, 1, 0, 1, 0, 10, 0)     // User 1 MARKET BUY to open position

	balances := ts.GetBalances(1)
	var initialAvailable int64
	for _, bal := range balances {
		if bal.Asset == "USDT" {
			initialAvailable = bal.Available
		}
	}

	err := ts.EditLeverage(1, 1, 10)
	if err != nil {
		t.Errorf("EditLeverage failed: %v", err)
	}

	balances = ts.GetBalances(1)
	var newAvailable int64
	for _, bal := range balances {
		if bal.Asset == "USDT" {
			newAvailable = bal.Available
		}
	}

	if newAvailable <= initialAvailable {
		t.Errorf("expected available to increase, got %d (was %d)", newAvailable, initialAvailable)
	}
}
