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

func TestMatchFlow_SpotPartialFills(t *testing.T) {
	port := portfolio.New(portfolio.Config{})
	clear := clearing.New(port)
	s, _ := New(Config{}, port, clear)

	ob := orderbook.New()
	s.orderbooks[constants.CATEGORY_SPOT]["BTCUSDT"] = ob

	makerID := types.UserID(2)
	port.Balances[makerID] = map[string]*types.UserBalance{
		"BTC":  {Asset: "BTC", Available: 0, Locked: 2},
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
	ob.Add(&types.Order{
		ID:       2,
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
	if result.Filled != 2 || result.Remaining != 0 {
		t.Fatalf("expected filled 2 remaining 0, got filled %d remaining %d", result.Filled, result.Remaining)
	}

	if got := port.Balances[takerID]["USDT"]; got.Available != 800 || got.Locked != 0 {
		t.Fatalf("taker USDT expected available 800 locked 0, got %+v", got)
	}
	if got := port.Balances[takerID]["BTC"]; got.Available != 2 {
		t.Fatalf("taker BTC expected 2, got %+v", got)
	}
	if got := port.Balances[makerID]["BTC"]; got.Locked != 0 || got.Available != 0 {
		t.Fatalf("maker BTC expected locked 0 available 0, got %+v", got)
	}
	if got := port.Balances[makerID]["USDT"]; got.Available != 200 {
		t.Fatalf("maker USDT expected 200, got %+v", got)
	}
}

func TestMatchFlow_LinearPartialFills(t *testing.T) {
	port := portfolio.New(portfolio.Config{})
	clear := clearing.New(port)
	s, _ := New(Config{}, port, clear)

	ob := orderbook.New()
	s.orderbooks[constants.CATEGORY_LINEAR]["BTCUSDT"] = ob

	makerID := types.UserID(2)
	port.Balances[makerID] = map[string]*types.UserBalance{
		"USDT": {Asset: "USDT", Available: 0, Locked: 4000},
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
		Quantity: 2,
	})

	takerID := types.UserID(1)
	port.Balances[takerID] = map[string]*types.UserBalance{
		"USDT": {Asset: "USDT", Available: 4000, Locked: 0, Margin: 0},
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
	if result.Filled != 2 || result.Remaining != 0 {
		t.Fatalf("expected filled 2 remaining 0, got filled %d remaining %d", result.Filled, result.Remaining)
	}

	if got := port.Balances[takerID]["USDT"]; got.Locked != 0 || got.Margin != 4000 {
		t.Fatalf("taker USDT expected locked 0 margin 4000, got %+v", got)
	}
	if got := port.Balances[makerID]["USDT"]; got.Locked != 0 || got.Margin != 4000 {
		t.Fatalf("maker USDT expected locked 0 margin 4000, got %+v", got)
	}
	if pos := port.Positions[takerID]["BTCUSDT"]; pos.Size != 2 || pos.Side != constants.ORDER_SIDE_BUY {
		t.Fatalf("taker position unexpected: %+v", pos)
	}
	if pos := port.Positions[makerID]["BTCUSDT"]; pos.Size != -2 || pos.Side != constants.ORDER_SIDE_SELL {
		t.Fatalf("maker position unexpected: %+v", pos)
	}
}

func TestMatchFlow_GTCLeavesRestingOrder(t *testing.T) {
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
		TIF:      constants.TIF_GTC,
		Quantity: 3,
		Price:    100,
	}

	result, err := s.PlaceOrder(context.Background(), input)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Status != constants.ORDER_STATUS_NEW {
		t.Fatalf("expected status NEW (resting), got %d", result.Status)
	}
	if result.Filled != 1 || result.Remaining != 2 {
		t.Fatalf("expected filled 1 remaining 2, got filled %d remaining %d", result.Filled, result.Remaining)
	}

	bidPrice, bidQty, _, _ := s.GetOrderBook(constants.CATEGORY_SPOT, "BTCUSDT")
	if bidPrice != 100 || bidQty != 2 {
		t.Fatalf("expected resting bid at 100 qty 2, got price %d qty %d", bidPrice, bidQty)
	}
}
