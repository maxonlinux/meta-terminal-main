package oms

import (
	"context"
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func TestOnPriceTick_ConditionalTriggers(t *testing.T) {
	clearing := &countingClearing{}
	s, _ := newTestServiceWithClearing(clearing)

	input := &types.OrderInput{
		UserID:       1,
		Symbol:       "BTCUSDT",
		Category:     constants.CATEGORY_LINEAR,
		Side:         constants.ORDER_SIDE_BUY,
		Type:         constants.ORDER_TYPE_LIMIT,
		TIF:          constants.TIF_GTC,
		Quantity:     2,
		Price:        100,
		TriggerPrice: 90,
	}

	result, err := s.PlaceOrder(context.Background(), input)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Status != constants.ORDER_STATUS_UNTRIGGERED {
		t.Fatalf("expected UNTRIGGERED, got %d", result.Status)
	}
	if s.triggerMon.Count() != 1 {
		t.Fatalf("expected 1 trigger stored, got %d", s.triggerMon.Count())
	}

	s.OnPriceTick("BTCUSDT", 90)

	if clearing.reserveCalls != 1 {
		t.Fatalf("expected reserve called once, got %d", clearing.reserveCalls)
	}
	if clearing.lastQty != 2 {
		t.Fatalf("expected reserve qty 2, got %d", clearing.lastQty)
	}
	if s.triggerMon.Count() != 0 {
		t.Fatalf("expected trigger removed, got %d", s.triggerMon.Count())
	}
}

func TestOnPriceTick_CloseOnTriggerUsesPositionSize(t *testing.T) {
	clearing := &countingClearing{}
	s, portfolio := newTestServiceWithClearing(clearing)
	portfolio.addPosition(1, "BTCUSDT", 3, constants.SIDE_LONG)

	input := &types.OrderInput{
		UserID:         1,
		Symbol:         "BTCUSDT",
		Category:       constants.CATEGORY_LINEAR,
		Side:           constants.ORDER_SIDE_BUY,
		Type:           constants.ORDER_TYPE_LIMIT,
		TIF:            constants.TIF_GTC,
		Quantity:       0,
		Price:          95,
		TriggerPrice:   90,
		CloseOnTrigger: true,
	}

	result, err := s.PlaceOrder(context.Background(), input)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Status != constants.ORDER_STATUS_UNTRIGGERED {
		t.Fatalf("expected UNTRIGGERED, got %d", result.Status)
	}

	s.OnPriceTick("BTCUSDT", 90)

	if clearing.reserveCalls != 1 {
		t.Fatalf("expected reserve called once, got %d", clearing.reserveCalls)
	}
	if clearing.lastQty != 3 {
		t.Fatalf("expected reserve qty from position size 3, got %d", clearing.lastQty)
	}
	if clearing.lastSide != constants.ORDER_SIDE_SELL {
		t.Fatalf("expected child side SELL, got %d", clearing.lastSide)
	}
}

func TestOnPriceTick_CloseOnTriggerOppositeSide(t *testing.T) {
	clearing := &countingClearing{}
	s, portfolio := newTestServiceWithClearing(clearing)
	portfolio.addPosition(1, "BTCUSDT", 4, constants.SIDE_SHORT)

	input := &types.OrderInput{
		UserID:         1,
		Symbol:         "BTCUSDT",
		Category:       constants.CATEGORY_LINEAR,
		Side:           constants.ORDER_SIDE_SELL,
		Type:           constants.ORDER_TYPE_LIMIT,
		TIF:            constants.TIF_GTC,
		Quantity:       0,
		Price:          105,
		TriggerPrice:   110,
		CloseOnTrigger: true,
	}

	_, err := s.PlaceOrder(context.Background(), input)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	s.OnPriceTick("BTCUSDT", 110)

	if clearing.reserveCalls != 1 {
		t.Fatalf("expected reserve called once, got %d", clearing.reserveCalls)
	}
	if clearing.lastQty != 4 {
		t.Fatalf("expected reserve qty from position size 4, got %d", clearing.lastQty)
	}
	if clearing.lastSide != constants.ORDER_SIDE_BUY {
		t.Fatalf("expected child side BUY for short close, got %d", clearing.lastSide)
	}
}
