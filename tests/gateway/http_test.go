package gateway_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/anomalyco/meta-terminal-go/internal/engine"
	"github.com/anomalyco/meta-terminal-go/internal/gateway"
	"github.com/anomalyco/meta-terminal-go/internal/registry"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

type stubHistory struct{}

func (stubHistory) GetOrderHistory(context.Context, types.UserID, string, int) ([]*types.Order, error) {
	return nil, nil
}
func (stubHistory) GetTradeHistory(context.Context, types.UserID, string, int) ([]*types.Trade, error) {
	return nil, nil
}
func (stubHistory) GetRPNLHistory(context.Context, types.UserID, string, int) ([]*types.RPNLEvent, error) {
	return nil, nil
}

func TestGatewayOrderbookRequiresParams(t *testing.T) {
	handler := newGatewayHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/orderbook", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestGatewayBalancesAuth(t *testing.T) {
	handler := newGatewayHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/balances", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestGatewayBalancesWithAuth(t *testing.T) {
	e, handler := newGatewayHandlerWithEngine(t)

	userID := types.UserID(42)
	if e.Portfolio.Balances[userID] == nil {
		e.Portfolio.Balances[userID] = map[string]*types.UserBalance{}
	}
	e.Portfolio.Balances[userID]["USDT"] = &types.UserBalance{
		Asset:     "USDT",
		Available: 1000,
		Locked:    0,
		Margin:    0,
	}

	token := makeJWT([]byte("secret"), userID)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/balances", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var balances []*types.UserBalance
	if err := json.NewDecoder(rec.Body).Decode(&balances); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(balances) != 1 || balances[0].Asset != "USDT" {
		t.Fatalf("unexpected balances response: %+v", balances)
	}
}

func TestGatewayOrderbookResponse(t *testing.T) {
	e, handler := newGatewayHandlerWithEngine(t)

	userID := types.UserID(1)
	if e.Portfolio.Balances[userID] == nil {
		e.Portfolio.Balances[userID] = map[string]*types.UserBalance{}
	}
	e.Portfolio.Balances[userID]["BTC"] = &types.UserBalance{Asset: "BTC", Available: 1}

	_, err := e.OMS.PlaceOrder(context.Background(), &types.OrderInput{
		UserID:   userID,
		Symbol:   "BTCUSDT",
		Category: 0,
		Side:     1,
		Type:     0,
		TIF:      0,
		Quantity: 1,
		Price:    100,
	})
	if err != nil {
		t.Fatalf("place order: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/orderbook?symbol=BTCUSDT&category=spot&limit=1", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var payload struct {
		Symbol string     `json:"s"`
		Bids   [][]string `json:"b"`
		Asks   [][]string `json:"a"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload.Symbol != "BTCUSDT" {
		t.Fatalf("expected symbol BTCUSDT, got %s", payload.Symbol)
	}
	if len(payload.Asks) != 1 {
		t.Fatalf("expected 1 ask, got %+v", payload.Asks)
	}
}

func TestGatewayPlaceOrderWithAuth(t *testing.T) {
	e, handler := newGatewayHandlerWithEngine(t)

	userID := types.UserID(99)
	if e.Portfolio.Balances[userID] == nil {
		e.Portfolio.Balances[userID] = map[string]*types.UserBalance{}
	}
	e.Portfolio.Balances[userID]["USDT"] = &types.UserBalance{
		Asset:     "USDT",
		Available: 100000,
	}

	body, _ := json.Marshal(&types.OrderInput{
		Symbol:   "BTCUSDT",
		Category: 0,
		Side:     0,
		Type:     0,
		TIF:      0,
		Quantity: 1,
		Price:    1000,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/order", bytes.NewReader(body))
	req.AddCookie(&http.Cookie{Name: "token", Value: makeJWT([]byte("secret"), userID)})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var result types.OrderResult
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(result.Orders) != 1 || result.Orders[0].UserID != userID {
		t.Fatalf("unexpected order result: %+v", result.Orders)
	}
}

func newGatewayHandler(t *testing.T) http.Handler {
	_, handler := newGatewayHandlerWithEngine(t)
	return handler
}

func newGatewayHandlerWithEngine(t *testing.T) (*engine.Engine, http.Handler) {
	t.Helper()
	dir := t.TempDir()
	env := engine.Config{
		OutboxPath: filepath.Join(dir, "outbox.log"),
		OutboxBuf:  32 * 1024,
	}

	e, err := engine.New(env)
	if err != nil {
		t.Fatalf("engine init: %v", err)
	}
	t.Cleanup(func() {
		_ = e.Close()
	})

	reg := registry.New()
	gw := gateway.New(gateway.Config{
		JWTSecret: "secret",
		JWTCookie: "token",
	}, e.OMS, e.Portfolio, stubHistory{}, reg)

	return e, gw.Handler()
}

func makeJWT(secret []byte, userID types.UserID) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256"}`))
	payloadData, _ := json.Marshal(map[string]any{
		"userID": uint64(userID),
		"exp":    time.Now().Add(5 * time.Minute).Unix(),
	})
	payload := base64.RawURLEncoding.EncodeToString(payloadData)
	signing := header + "." + payload
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(signing))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return signing + "." + sig
}
