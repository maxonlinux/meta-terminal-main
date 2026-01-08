package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
)

func newTestServer() (*httptest.Server, *engine.Engine) {
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
	apiServer := NewServer(eng, nil, reg)
	mux := http.NewServeMux()
	apiServer.Register(mux)
	return httptest.NewServer(mux), eng
}

func TestAPIFlow(t *testing.T) {
	srv, eng := newTestServer()
	defer srv.Close()

	eng.AddInstrument("BTCUSDT", constants.CATEGORY_SPOT, 50000)
	if err := eng.SetBalance(1, "USDT", 1_000_000); err != nil {
		t.Fatalf("set balance failed: %v", err)
	}

	orderPayload := map[string]any{
		"userId":   1,
		"symbol":   "BTCUSDT",
		"category": "SPOT",
		"side":     "BUY",
		"type":     "LIMIT",
		"tif":      "GTC",
		"qty":      "10",
		"price":    "50000",
	}
	body, _ := json.Marshal(orderPayload)
	resp, err := http.Post(srv.URL+"/trading/order", "application/json", bytes.NewReader(body))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("place order failed")
	}
	var orderResp struct {
		Order struct {
			ID uint64 `json:"id"`
		} `json:"order"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&orderResp); err != nil {
		t.Fatalf("decode order response failed: %v", err)
	}
	_ = resp.Body.Close()

	resp, err = http.Get(srv.URL + "/open-orders?userId=1")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("open orders failed")
	}

	cancelPayload := map[string]any{
		"userId":  1,
		"orderId": orderResp.Order.ID,
	}
	body, _ = json.Marshal(cancelPayload)
	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/trading/order", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("cancel order failed")
	}

	resp, err = http.Get(srv.URL + "/trading/balance?userId=1&asset=USDT")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("balance failed")
	}

	resp, err = http.Get(srv.URL + "/trading/spot/BTCUSDT/instrument")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("instrument failed")
	}

	resp, err = http.Get(srv.URL + "/trading/spot/BTCUSDT/orders/history?userId=1&limit=10")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("orders history failed")
	}

	resp, err = http.Get(srv.URL + "/trading/spot/BTCUSDT/trades?userId=1&limit=10")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("trades history failed")
	}
}

func TestParseCategory(t *testing.T) {
	cat, err := parseCategory("SPOT")
	if err != nil || cat != constants.CATEGORY_SPOT {
		t.Fatalf("expected spot")
	}
}
