package oms_test

import (
	"context"
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

type countingClearing struct {
	reserveCalls int
	lastQty      types.Quantity
	lastPrice    types.Price
	lastSide     int8
}

func (c *countingClearing) Reserve(userID types.UserID, symbol string, category int8, side int8, qty types.Quantity, price types.Price) error {
	c.reserveCalls++
	c.lastQty = qty
	c.lastPrice = price
	c.lastSide = side
	return nil
}

func (c *countingClearing) Release(userID types.UserID, symbol string, category int8, side int8, qty types.Quantity, price types.Price) {
}

func (c *countingClearing) ExecuteTrade(trade *types.Trade, taker *types.Order, maker *types.Order) {
}

func TestOnPriceTickConditionalTriggers(t *testing.T) {
	clearing := &countingClearing{}
	s, _ := newServiceWithClearing(clearing)

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

	s.OnPriceTick("BTCUSDT", 90)

	if clearing.reserveCalls != 1 {
		t.Fatalf("expected reserve called once, got %d", clearing.reserveCalls)
	}
	if clearing.lastQty != 2 {
		t.Fatalf("expected reserve qty 2, got %d", clearing.lastQty)
	}
}

func TestOnPriceTickCloseOnTriggerUsesPositionSize(t *testing.T) {
	clearing := &countingClearing{}
	s, port := newServiceWithClearing(clearing)
	setPosition(port, 1, "BTCUSDT", 3, constants.SIDE_LONG, 50000, 10)

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

func TestOnPriceTickCloseOnTriggerOppositeSide(t *testing.T) {
	clearing := &countingClearing{}
	s, port := newServiceWithClearing(clearing)
	setPosition(port, 1, "BTCUSDT", 4, constants.SIDE_SHORT, 50000, 10)

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

func TestTriggerRulesBuyAndSell(t *testing.T) {
	s, _ := newService()
	s.OnPriceTick("BTCUSDT", 100)

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
	if _, err := s.PlaceOrder(context.Background(), buy); err != nil {
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
	if _, err := s.PlaceOrder(context.Background(), sell); err != nil {
		t.Fatalf("expected valid sell trigger above current price, got %v", err)
	}
}
