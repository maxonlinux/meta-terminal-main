package oms_test

import (
	"context"
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func TestMatchFlowSpotPartialFills(t *testing.T) {
	s, port := newService()

	makerID := types.UserID(2)
	setBalance(port, makerID, "BTC", 2, 0, 0)
	setBalance(port, makerID, "USDT", 0, 0, 0)

	for i := 0; i < 2; i++ {
		_, err := s.PlaceOrder(context.Background(), &types.OrderInput{
			UserID:   makerID,
			Symbol:   "BTCUSDT",
			Category: constants.CATEGORY_SPOT,
			Side:     constants.ORDER_SIDE_SELL,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Quantity: 1,
			Price:    100,
		})
		if err != nil {
			t.Fatalf("maker place error: %v", err)
		}
	}

	takerID := types.UserID(1)
	setBalance(port, takerID, "USDT", 1000, 0, 0)
	setBalance(port, takerID, "BTC", 0, 0, 0)

	result, err := s.PlaceOrder(context.Background(), &types.OrderInput{
		UserID:   takerID,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_SPOT,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_IOC,
		Quantity: 2,
		Price:    100,
	})
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
	if got := port.Balances[makerID]["BTC"]; got.Locked != 0 {
		t.Fatalf("maker BTC expected locked 0, got %+v", got)
	}
	if got := port.Balances[makerID]["USDT"]; got.Available != 200 {
		t.Fatalf("maker USDT expected 200, got %+v", got)
	}
}

func TestMatchFlowLinearPartialFills(t *testing.T) {
	s, port := newService()

	makerID := types.UserID(2)
	setBalance(port, makerID, "USDT", 4000, 0, 0)
	setPosition(port, makerID, "BTCUSDT", 0, constants.SIDE_NONE, 0, 5)

	_, err := s.PlaceOrder(context.Background(), &types.OrderInput{
		UserID:   makerID,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 2,
		Price:    10000,
	})
	if err != nil {
		t.Fatalf("maker place error: %v", err)
	}

	takerID := types.UserID(1)
	setBalance(port, takerID, "USDT", 4000, 0, 0)
	setPosition(port, takerID, "BTCUSDT", 0, constants.SIDE_NONE, 0, 5)

	result, err := s.PlaceOrder(context.Background(), &types.OrderInput{
		UserID:   takerID,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_IOC,
		Quantity: 2,
		Price:    10000,
	})
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

func TestMatchFlowGTCLeavesRestingOrder(t *testing.T) {
	s, port := newService()

	makerID := types.UserID(2)
	setBalance(port, makerID, "BTC", 1, 0, 0)

	_, err := s.PlaceOrder(context.Background(), &types.OrderInput{
		UserID:   makerID,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_SPOT,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 1,
		Price:    100,
	})
	if err != nil {
		t.Fatalf("maker place error: %v", err)
	}

	takerID := types.UserID(1)
	setBalance(port, takerID, "USDT", 1000, 0, 0)
	setBalance(port, takerID, "BTC", 0, 0, 0)

	result, err := s.PlaceOrder(context.Background(), &types.OrderInput{
		UserID:   takerID,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_SPOT,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 3,
		Price:    100,
	})
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

func TestIOCPartialRefundSpot(t *testing.T) {
	s, port := newService()

	makerID := types.UserID(2)
	setBalance(port, makerID, "BTC", 1, 0, 0)
	setBalance(port, makerID, "USDT", 0, 0, 0)

	_, err := s.PlaceOrder(context.Background(), &types.OrderInput{
		UserID:   makerID,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_SPOT,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 1,
		Price:    100,
	})
	if err != nil {
		t.Fatalf("maker place error: %v", err)
	}

	takerID := types.UserID(1)
	setBalance(port, takerID, "USDT", 1000, 0, 0)
	setBalance(port, takerID, "BTC", 0, 0, 0)

	result, err := s.PlaceOrder(context.Background(), &types.OrderInput{
		UserID:   takerID,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_SPOT,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_IOC,
		Quantity: 2,
		Price:    100,
	})
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

func TestIOCPartialRefundLinear(t *testing.T) {
	s, port := newService()

	makerID := types.UserID(2)
	setBalance(port, makerID, "USDT", 100000, 0, 0)
	setPosition(port, makerID, "BTCUSDT", 0, constants.SIDE_NONE, 0, 5)

	_, err := s.PlaceOrder(context.Background(), &types.OrderInput{
		UserID:   makerID,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 1,
		Price:    10000,
	})
	if err != nil {
		t.Fatalf("maker place error: %v", err)
	}

	takerID := types.UserID(1)
	setBalance(port, takerID, "USDT", 10000, 0, 0)
	setPosition(port, takerID, "BTCUSDT", 0, constants.SIDE_NONE, 0, 5)

	result, err := s.PlaceOrder(context.Background(), &types.OrderInput{
		UserID:   takerID,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_IOC,
		Quantity: 2,
		Price:    10000,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Filled != 1 || result.Remaining != 1 {
		t.Fatalf("expected filled 1 remaining 1, got filled %d remaining %d", result.Filled, result.Remaining)
	}

	if got := port.Balances[takerID]["USDT"]; got.Locked != 0 || got.Margin != 2000 || got.Available != 8000 {
		t.Fatalf("taker USDT expected avail 8000 margin 2000 locked 0, got %+v", got)
	}
	if got := port.Balances[makerID]["USDT"]; got.Locked != 0 || got.Margin != 2000 {
		t.Fatalf("maker USDT expected locked 0 margin 2000, got %+v", got)
	}
}

func TestPostOnlyRestingNoTrade(t *testing.T) {
	s, port := newService()

	makerID := types.UserID(2)
	setBalance(port, makerID, "USDT", 100000, 0, 0)
	setPosition(port, makerID, "BTCUSDT", 0, constants.SIDE_NONE, 0, 5)

	_, err := s.PlaceOrder(context.Background(), &types.OrderInput{
		UserID:   makerID,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 1,
		Price:    10500,
	})
	if err != nil {
		t.Fatalf("maker place error: %v", err)
	}

	userID := types.UserID(1)
	setBalance(port, userID, "USDT", 10000, 0, 0)
	setPosition(port, userID, "BTCUSDT", 0, constants.SIDE_NONE, 0, 5)

	result, err := s.PlaceOrder(context.Background(), &types.OrderInput{
		UserID:   userID,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_POST_ONLY,
		Quantity: 2,
		Price:    10000,
	})
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
	if got := port.Balances[userID]["USDT"]; got.Available != 6000 || got.Locked != 4000 {
		t.Fatalf("expected available 6000 locked 4000, got %+v", got)
	}
}
