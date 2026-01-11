package oms

import (
	"context"
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/orderbook"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func TestSelfMatchPrevention_NoTradeOccurs(t *testing.T) {
	s, _ := newTestService()

	ob := orderbook.New()
	s.orderbooks[constants.CATEGORY_LINEAR]["BTCUSDT"] = ob

	ownAsk := &types.Order{
		ID:       1,
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Status:   constants.ORDER_STATUS_NEW,
		Price:    100,
		Quantity: 1,
	}
	ob.Add(ownAsk)
	s.storeOrder(ownAsk)

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 1,
		Price:    100,
	}

	if _, err := s.PlaceOrder(context.Background(), input); err != ErrSelfMatch {
		t.Fatalf("expected ErrSelfMatch, got %v", err)
	}

	if ownAsk.Filled != 0 {
		t.Fatalf("expected no fill on own order, got filled %d", ownAsk.Filled)
	}
}

func TestTriggerRules_BuyAndSell(t *testing.T) {
	s, portfolio := newTestService()
	portfolio.addPosition(1, "BTCUSDT", 1, constants.SIDE_LONG)
	s.lastPrices["BTCUSDT"] = 100

	buy := &types.OrderInput{
		UserID:       1,
		Symbol:       "BTCUSDT",
		Category:     constants.CATEGORY_LINEAR,
		Side:         constants.ORDER_SIDE_BUY,
		Type:         constants.ORDER_TYPE_LIMIT,
		TIF:          constants.TIF_GTC,
		Quantity:     1,
		Price:        100,
		TriggerPrice: 99,
	}
	if err := s.validateOrder(buy); err != nil {
		t.Fatalf("expected valid buy trigger below current price, got %v", err)
	}

	sell := &types.OrderInput{
		UserID:       1,
		Symbol:       "BTCUSDT",
		Category:     constants.CATEGORY_LINEAR,
		Side:         constants.ORDER_SIDE_SELL,
		Type:         constants.ORDER_TYPE_LIMIT,
		TIF:          constants.TIF_GTC,
		Quantity:     1,
		Price:        100,
		TriggerPrice: 101,
	}
	if err := s.validateOrder(sell); err != nil {
		t.Fatalf("expected valid sell trigger above current price, got %v", err)
	}
}
