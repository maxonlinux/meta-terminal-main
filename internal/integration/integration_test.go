package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	httpapi "github.com/anomalyco/meta-terminal-go/internal/api/http"
	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/engine"
	"github.com/anomalyco/meta-terminal-go/internal/history"
	"github.com/anomalyco/meta-terminal-go/internal/idgen"
	"github.com/anomalyco/meta-terminal-go/internal/linear"
	"github.com/anomalyco/meta-terminal-go/internal/market"
	"github.com/anomalyco/meta-terminal-go/internal/orderbook"
	"github.com/anomalyco/meta-terminal-go/internal/orderstore"
	"github.com/anomalyco/meta-terminal-go/internal/registry"
	"github.com/anomalyco/meta-terminal-go/internal/spot"
	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

type testServer struct {
	server *httptest.Server
	eng    *engine.Engine
}

func newTestServer(t *testing.T) *testServer {
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
	apiServer := httpapi.NewServer(eng, nil, reg)
	mux := http.NewServeMux()
	apiServer.Register(mux)

	return &testServer{server: httptest.NewServer(mux), eng: eng}
}

func (ts *testServer) Close() { ts.server.Close() }

func (ts *testServer) url(path string) string { return ts.server.URL + path }

func (ts *testServer) addInstrument(t *testing.T, symbol string, category string, price string) {
	cat, err := parseCategory(category)
	if err != nil {
		t.Fatalf("invalid category: %v", err)
	}
	val, err := strconv.ParseInt(price, 10, 64)
	if err != nil {
		t.Fatalf("invalid price: %v", err)
	}
	ts.eng.AddInstrument(symbol, cat, types.Price(val))
}

func (ts *testServer) setBalance(t *testing.T, userID int, asset string, amount string) {
	val, err := strconv.ParseInt(amount, 10, 64)
	if err != nil {
		t.Fatalf("invalid amount: %v", err)
	}
	if err := ts.eng.SetBalance(types.UserID(userID), asset, val); err != nil {
		t.Fatalf("set balance failed: %v", err)
	}
}

type orderResp struct {
	Order struct {
		ID     uint64 `json:"id"`
		Status int8   `json:"status"`
		Filled string `json:"filled"`
	} `json:"order"`
	Status int8 `json:"status"`
}

func (ts *testServer) placeOrder(t *testing.T, payload map[string]any) orderResp {
	body, _ := json.Marshal(payload)
	resp, err := http.Post(ts.url("/trading/order"), "application/json", bytes.NewReader(body))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("place order failed: %v", err)
	}
	defer resp.Body.Close()
	var out orderResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode order response failed: %v", err)
	}
	return out
}

func filledFrom(resp orderResp) int64 {
	n, _ := strconv.ParseInt(resp.Order.Filled, 10, 64)
	return n
}

func parseCategory(val string) (int8, error) {
	switch val {
	case "SPOT":
		return constants.CATEGORY_SPOT, nil
	case "LINEAR":
		return constants.CATEGORY_LINEAR, nil
	default:
		return 0, http.ErrNotSupported
	}
}

func (ts *testServer) cancelOrder(t *testing.T, userID int, orderID uint64) {
	body, _ := json.Marshal(map[string]any{"userId": userID, "orderId": orderID})
	req, _ := http.NewRequest(http.MethodDelete, ts.url("/trading/order"), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("cancel order failed: %v", err)
	}
	_ = resp.Body.Close()
}

func (ts *testServer) getPosition(t *testing.T, userID int, symbol string) engine.PositionSnapshot {
	resp, err := http.Get(ts.url("/trading/position?userId=" + itoa(userID) + "&symbol=" + symbol))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("position failed: %v", err)
	}
	defer resp.Body.Close()
	var pos engine.PositionSnapshot
	if err := json.NewDecoder(resp.Body).Decode(&pos); err != nil {
		t.Fatalf("decode position failed: %v", err)
	}
	return pos
}

func (ts *testServer) getBalances(t *testing.T, userID int) []engine.BalanceSnapshot {
	resp, err := http.Get(ts.url("/trading/balances?userId=" + itoa(userID)))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("balances failed: %v", err)
	}
	defer resp.Body.Close()
	var balances []engine.BalanceSnapshot
	if err := json.NewDecoder(resp.Body).Decode(&balances); err != nil {
		t.Fatalf("decode balances failed: %v", err)
	}
	return balances
}

func (ts *testServer) editLeverage(t *testing.T, userID int, symbol string, leverage int) {
	body, _ := json.Marshal(map[string]any{"userId": userID, "symbol": symbol, "leverage": leverage})
	req, _ := http.NewRequest(http.MethodPatch, ts.url("/trading/position/leverage"), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("edit leverage failed: %v", err)
	}
	_ = resp.Body.Close()
}

func TestIntegrationFullSpotFlow(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	ts.addInstrument(t, "BTCUSDT", "SPOT", "50000")
	ts.setBalance(t, 1, "USDT", "1000000")

	resp := ts.placeOrder(t, map[string]any{
		"userId":   1,
		"symbol":   "BTCUSDT",
		"category": "SPOT",
		"side":     "BUY",
		"type":     "LIMIT",
		"tif":      "GTC",
		"qty":      "10",
		"price":    "50000",
	})
	if resp.Status != constants.ORDER_STATUS_NEW {
		t.Fatalf("expected NEW, got %d", resp.Status)
	}
}

func TestIntegrationLinearOpenPosition(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	ts.addInstrument(t, "BTCUSDT", "LINEAR", "50000")
	ts.setBalance(t, 1, "USDT", "1000000")
	ts.setBalance(t, 2, "USDT", "1000000")

	_ = ts.placeOrder(t, map[string]any{
		"userId":   1,
		"symbol":   "BTCUSDT",
		"category": "LINEAR",
		"side":     "BUY",
		"type":     "LIMIT",
		"tif":      "GTC",
		"qty":      "10",
		"price":    "50000",
		"leverage": 2,
	})
	resp := ts.placeOrder(t, map[string]any{
		"userId":   2,
		"symbol":   "BTCUSDT",
		"category": "LINEAR",
		"side":     "SELL",
		"type":     "MARKET",
		"tif":      "IOC",
		"qty":      "10",
	})
	if resp.Status != constants.ORDER_STATUS_FILLED {
		t.Fatalf("expected FILLED, got %d", resp.Status)
	}

	pos := ts.getPosition(t, 1, "BTCUSDT")
	if pos.Size != 10 {
		t.Fatalf("expected size 10, got %d", pos.Size)
	}
}

func TestIntegrationTwoUsersTrade(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	ts.addInstrument(t, "BTCUSDT", "LINEAR", "50000")
	ts.setBalance(t, 1, "USDT", "1000000")
	ts.setBalance(t, 2, "USDT", "1000000")

	_ = ts.placeOrder(t, map[string]any{
		"userId":   1,
		"symbol":   "BTCUSDT",
		"category": "LINEAR",
		"side":     "BUY",
		"type":     "LIMIT",
		"tif":      "GTC",
		"qty":      "10",
		"price":    "50000",
		"leverage": 2,
	})
	resp := ts.placeOrder(t, map[string]any{
		"userId":   2,
		"symbol":   "BTCUSDT",
		"category": "LINEAR",
		"side":     "SELL",
		"type":     "MARKET",
		"tif":      "IOC",
		"qty":      "10",
	})
	if resp.Status != constants.ORDER_STATUS_FILLED {
		t.Fatalf("expected FILLED, got %d", resp.Status)
	}

	pos1 := ts.getPosition(t, 1, "BTCUSDT")
	if pos1.Size != 10 {
		t.Fatalf("user1 expected size 10")
	}
	pos2 := ts.getPosition(t, 2, "BTCUSDT")
	if pos2.Size != 10 || pos2.Side != constants.SIDE_SHORT {
		t.Fatalf("user2 expected short size 10")
	}
}

func TestIntegrationEditLeverage(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	ts.addInstrument(t, "BTCUSDT", "LINEAR", "50000")
	ts.setBalance(t, 1, "USDT", "1000000")

	ts.editLeverage(t, 1, "BTCUSDT", 10)
	pos := ts.getPosition(t, 1, "BTCUSDT")
	if pos.Leverage != 10 {
		t.Fatalf("expected leverage 10, got %d", pos.Leverage)
	}
}

func TestIntegrationCancelOrder(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	ts.addInstrument(t, "BTCUSDT", "SPOT", "50000")
	ts.setBalance(t, 1, "USDT", "1000000")

	resp := ts.placeOrder(t, map[string]any{
		"userId":   1,
		"symbol":   "BTCUSDT",
		"category": "SPOT",
		"side":     "BUY",
		"type":     "LIMIT",
		"tif":      "GTC",
		"qty":      "10",
		"price":    "50000",
	})
	ts.cancelOrder(t, 1, resp.Order.ID)
}

func TestIntegrationMultipleOrdersSamePrice(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	ts.addInstrument(t, "BTCUSDT", "LINEAR", "50000")
	ts.setBalance(t, 1, "USDT", "1000000")
	ts.setBalance(t, 2, "USDT", "1000000")
	ts.setBalance(t, 3, "USDT", "1000000")

	_ = ts.placeOrder(t, map[string]any{"userId": 1, "symbol": "BTCUSDT", "category": "LINEAR", "side": "BUY", "type": "LIMIT", "tif": "GTC", "qty": "5", "price": "50000", "leverage": 2})
	_ = ts.placeOrder(t, map[string]any{"userId": 2, "symbol": "BTCUSDT", "category": "LINEAR", "side": "BUY", "type": "LIMIT", "tif": "GTC", "qty": "5", "price": "50000", "leverage": 2})
	resp := ts.placeOrder(t, map[string]any{"userId": 3, "symbol": "BTCUSDT", "category": "LINEAR", "side": "SELL", "type": "MARKET", "tif": "IOC", "qty": "8"})

	if filledFrom(resp) != 8 {
		t.Fatalf("expected filled 8, got %d", filledFrom(resp))
	}
	pos := ts.getPosition(t, 3, "BTCUSDT")
	if pos.Size != 8 || pos.Side != constants.SIDE_SHORT {
		t.Fatalf("expected short size 8")
	}
}

func TestIntegrationPartialFill(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	ts.addInstrument(t, "BTCUSDT", "LINEAR", "50000")
	ts.setBalance(t, 1, "USDT", "1000000")
	ts.setBalance(t, 2, "USDT", "1000000")

	_ = ts.placeOrder(t, map[string]any{"userId": 1, "symbol": "BTCUSDT", "category": "LINEAR", "side": "BUY", "type": "LIMIT", "tif": "GTC", "qty": "5", "price": "50000", "leverage": 2})
	resp := ts.placeOrder(t, map[string]any{"userId": 2, "symbol": "BTCUSDT", "category": "LINEAR", "side": "SELL", "type": "MARKET", "tif": "IOC", "qty": "10"})

	if resp.Status != constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED {
		t.Fatalf("expected PARTIALLY_FILLED_CANCELED, got %d", resp.Status)
	}
	if filledFrom(resp) != 5 {
		t.Fatalf("expected filled 5, got %d", filledFrom(resp))
	}
}

func TestIntegrationReduceOnly(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	ts.addInstrument(t, "BTCUSDT", "LINEAR", "50000")
	ts.setBalance(t, 1, "USDT", "1000000")
	ts.setBalance(t, 2, "USDT", "1000000")

	_ = ts.placeOrder(t, map[string]any{"userId": 2, "symbol": "BTCUSDT", "category": "LINEAR", "side": "SELL", "type": "LIMIT", "tif": "GTC", "qty": "10", "price": "50000", "leverage": 2})
	_ = ts.placeOrder(t, map[string]any{"userId": 1, "symbol": "BTCUSDT", "category": "LINEAR", "side": "BUY", "type": "MARKET", "tif": "IOC", "qty": "10"})
	_ = ts.placeOrder(t, map[string]any{"userId": 2, "symbol": "BTCUSDT", "category": "LINEAR", "side": "BUY", "type": "LIMIT", "tif": "GTC", "qty": "5", "price": "50000", "leverage": 2})

	resp := ts.placeOrder(t, map[string]any{"userId": 1, "symbol": "BTCUSDT", "category": "LINEAR", "side": "SELL", "type": "MARKET", "tif": "IOC", "qty": "5", "reduceOnly": true})
	if resp.Status != constants.ORDER_STATUS_FILLED {
		t.Fatalf("expected FILLED, got %d", resp.Status)
	}
	pos := ts.getPosition(t, 1, "BTCUSDT")
	if pos.Size != 5 {
		t.Fatalf("expected size 5, got %d", pos.Size)
	}
}

func TestIntegrationBalanceTracking(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	ts.addInstrument(t, "BTCUSDT", "SPOT", "50000")
	ts.setBalance(t, 1, "USDT", "1000000")

	_ = ts.placeOrder(t, map[string]any{"userId": 1, "symbol": "BTCUSDT", "category": "SPOT", "side": "BUY", "type": "LIMIT", "tif": "GTC", "qty": "10", "price": "50000"})

	balances := ts.getBalances(t, 1)
	var locked int64
	for _, b := range balances {
		if b.Asset == "USDT" {
			locked = b.Locked
		}
	}
	if locked != 10*50000 {
		t.Fatalf("expected locked %d, got %d", 10*50000, locked)
	}
}

func TestIntegrationIOCOrder(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	ts.addInstrument(t, "BTCUSDT", "LINEAR", "50000")
	ts.setBalance(t, 1, "USDT", "1000000")
	ts.setBalance(t, 2, "USDT", "1000000")

	_ = ts.placeOrder(t, map[string]any{"userId": 1, "symbol": "BTCUSDT", "category": "LINEAR", "side": "BUY", "type": "LIMIT", "tif": "GTC", "qty": "5", "price": "50000", "leverage": 2})
	resp := ts.placeOrder(t, map[string]any{"userId": 2, "symbol": "BTCUSDT", "category": "LINEAR", "side": "SELL", "type": "MARKET", "tif": "IOC", "qty": "100"})

	if resp.Status != constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED {
		t.Fatalf("expected PARTIALLY_FILLED_CANCELED, got %d", resp.Status)
	}
	if filledFrom(resp) != 5 {
		t.Fatalf("expected filled 5, got %d", filledFrom(resp))
	}
}

func TestIntegrationPositionSideUpdates(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	ts.addInstrument(t, "BTCUSDT", "LINEAR", "50000")
	ts.setBalance(t, 1, "USDT", "1000000")
	ts.setBalance(t, 2, "USDT", "1000000")

	_ = ts.placeOrder(t, map[string]any{"userId": 2, "symbol": "BTCUSDT", "category": "LINEAR", "side": "SELL", "type": "LIMIT", "tif": "GTC", "qty": "20", "price": "50000", "leverage": 2})
	_ = ts.placeOrder(t, map[string]any{"userId": 2, "symbol": "BTCUSDT", "category": "LINEAR", "side": "BUY", "type": "LIMIT", "tif": "GTC", "qty": "20", "price": "49000", "leverage": 2})

	_ = ts.placeOrder(t, map[string]any{"userId": 1, "symbol": "BTCUSDT", "category": "LINEAR", "side": "SELL", "type": "MARKET", "tif": "IOC", "qty": "10"})
	pos := ts.getPosition(t, 1, "BTCUSDT")
	if pos.Side != constants.SIDE_SHORT || pos.Size != 10 {
		t.Fatalf("expected short size 10")
	}

	_ = ts.placeOrder(t, map[string]any{"userId": 1, "symbol": "BTCUSDT", "category": "LINEAR", "side": "BUY", "type": "MARKET", "tif": "IOC", "qty": "15"})
	pos = ts.getPosition(t, 1, "BTCUSDT")
	if pos.Side != constants.SIDE_LONG || pos.Size != 5 {
		t.Fatalf("expected long size 5")
	}
}

func TestIntegrationLeverageChange(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	ts.addInstrument(t, "BTCUSDT", "LINEAR", "50000")
	ts.setBalance(t, 1, "USDT", "1000000")
	ts.setBalance(t, 2, "USDT", "1000000")

	_ = ts.placeOrder(t, map[string]any{"userId": 2, "symbol": "BTCUSDT", "category": "LINEAR", "side": "SELL", "type": "LIMIT", "tif": "GTC", "qty": "10", "price": "50000", "leverage": 2})
	_ = ts.placeOrder(t, map[string]any{"userId": 1, "symbol": "BTCUSDT", "category": "LINEAR", "side": "BUY", "type": "MARKET", "tif": "IOC", "qty": "10"})

	pos := ts.getPosition(t, 1, "BTCUSDT")
	if pos.Size != 10 {
		t.Fatalf("expected size 10")
	}

	ts.editLeverage(t, 1, "BTCUSDT", 10)
	pos = ts.getPosition(t, 1, "BTCUSDT")
	if pos.Leverage != 10 {
		t.Fatalf("expected leverage 10")
	}
}

func itoa(v int) string {
	return strconv.Itoa(v)
}
