package oms

import (
	"context"
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/clearing"
	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/orderbook"
	"github.com/anomalyco/meta-terminal-go/internal/portfolio"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func TestIOCPartialRefund_Spot(t *testing.T) {
	port := portfolio.New(portfolio.Config{})
	clear := clearing.New(port)
	s, _ := New(Config{}, port, clear)

	ob := orderbook.New()
	s.orderbooks[constants.CATEGORY_SPOT]["BTCUSDT"] = ob

	makerID := types.UserID(2)
	port.Balances[makerID] = map[string]*types.UserBalance{
		"BTC":  {Asset: "BTC", Available: 0, Locked: 1},
		"USDT": {Asset: "USDT", Available: 0, Locked: 0},
	}
	ob.Add(&types.Order{
		ID:       1,
		UserID:   makerID,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_SPOT,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Status:   constants.ORDER_STATUS_NEW,
		Price:    100,
		Quantity: 1,
	})

	takerID := types.UserID(1)
	port.Balances[takerID] = map[string]*types.UserBalance{
		"USDT": {Asset: "USDT", Available: 1000, Locked: 0},
		"BTC":  {Asset: "BTC", Available: 0, Locked: 0},
	}

	input := &types.OrderInput{
		UserID:   takerID,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_SPOT,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_IOC,
		Quantity: 2,
		Price:    100,
	}

	result, err := s.PlaceOrder(context.Background(), input)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Filled != 1 || result.Remaining != 1 {
		t.Fatalf("expected filled 1 remaining 1, got filled %d remaining %d", result.Filled, result.Remaining)
	}

	if got := port.Balances[takerID]["USDT"]; got.Available != 900 || got.Locked != 0 {
		t.Fatalf("taker USDT expected available 900 locked 0, got %+v", got)
	}
	if got := port.Balances[takerID]["BTC"]; got.Available != 1 {
		t.Fatalf("taker BTC expected 1, got %+v", got)
	}
	if got := port.Balances[makerID]["BTC"]; got.Locked != 0 {
		t.Fatalf("maker BTC expected locked 0, got %+v", got)
	}
	if got := port.Balances[makerID]["USDT"]; got.Available != 100 {
		t.Fatalf("maker USDT expected 100, got %+v", got)
	}
}

func TestIOCPartialRefund_Linear(t *testing.T) {
	port := portfolio.New(portfolio.Config{})
	clear := clearing.New(port)
	s, _ := New(Config{}, port, clear)

	ob := orderbook.New()
	s.orderbooks[constants.CATEGORY_LINEAR]["BTCUSDT"] = ob

	makerID := types.UserID(2)
	port.Balances[makerID] = map[string]*types.UserBalance{
		"USDT": {Asset: "USDT", Available: 0, Locked: 4000, Margin: 0},
	}
	port.Positions[makerID] = map[string]*types.Position{
		"BTCUSDT": {Symbol: "BTCUSDT", Size: 0, Side: -1, Leverage: 5},
	}
	ob.Add(&types.Order{
		ID:       1,
		UserID:   makerID,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Status:   constants.ORDER_STATUS_NEW,
		Price:    10000,
		Quantity: 1,
	})

	takerID := types.UserID(1)
	port.Balances[takerID] = map[string]*types.UserBalance{
		"USDT": {Asset: "USDT", Available: 10000, Locked: 0, Margin: 0},
	}
	port.Positions[takerID] = map[string]*types.Position{
		"BTCUSDT": {Symbol: "BTCUSDT", Size: 0, Side: -1, Leverage: 5},
	}

	input := &types.OrderInput{
		UserID:   takerID,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_IOC,
		Quantity: 2,
		Price:    10000,
	}

	result, err := s.PlaceOrder(context.Background(), input)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Filled != 1 || result.Remaining != 1 {
		t.Fatalf("expected filled 1 remaining 1, got filled %d remaining %d", result.Filled, result.Remaining)
	}

	if got := port.Balances[takerID]["USDT"]; got.Locked != 0 || got.Margin != 2000 || got.Available != 8000 {
		t.Fatalf("taker USDT expected avail 8000 margin 2000 locked 0, got %+v", got)
	}
	if got := port.Balances[makerID]["USDT"]; got.Locked != 2000 || got.Margin != 2000 {
		t.Fatalf("maker USDT expected locked 2000 margin 2000, got %+v", got)
	}
}

func TestPostOnlyResting_NoTrade(t *testing.T) {
	port := portfolio.New(portfolio.Config{})
	clear := clearing.New(port)
	s, _ := New(Config{}, port, clear)

	ob := orderbook.New()
	s.orderbooks[constants.CATEGORY_LINEAR]["BTCUSDT"] = ob

	ob.Add(&types.Order{
		ID:       1,
		UserID:   2,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Status:   constants.ORDER_STATUS_NEW,
		Price:    10500,
		Quantity: 1,
	})

	userID := types.UserID(1)
	port.Balances[userID] = map[string]*types.UserBalance{
		"USDT": {Asset: "USDT", Available: 10000, Locked: 0, Margin: 0},
	}

	input := &types.OrderInput{
		UserID:   userID,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_POST_ONLY,
		Quantity: 2,
		Price:    10000,
	}

	result, err := s.PlaceOrder(context.Background(), input)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Status != constants.ORDER_STATUS_NEW {
		t.Fatalf("expected NEW status, got %d", result.Status)
	}
	if result.Filled != 0 || result.Remaining != 2 {
		t.Fatalf("expected filled 0 remaining 2, got filled %d remaining %d", result.Filled, result.Remaining)
	}

	bidPrice, bidQty, _, _ := s.GetOrderBook(constants.CATEGORY_LINEAR, "BTCUSDT")
	if bidPrice != 10000 || bidQty != 2 {
		t.Fatalf("expected resting bid at 10000 qty 2, got price %d qty %d", bidPrice, bidQty)
	}
	if got := port.Balances[userID]["USDT"]; got.Available != 0 || got.Locked != 10000 {
		t.Fatalf("expected available 0 locked 10000, got %+v", got)
	}
}
