package oms

import (
	"context"
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func TestCloseOnTrigger_ClampsQtyToPosition(t *testing.T) {
	clearing := &countingClearing{}
	s, portfolio := newTestServiceWithClearing(clearing)
	portfolio.addPosition(1, "BTCUSDT", 3, constants.SIDE_LONG)

	input := &types.OrderInput{
		UserID:         1,
		Symbol:         "BTCUSDT",
		Category:       constants.CATEGORY_LINEAR,
		Side:           constants.ORDER_SIDE_SELL,
		Type:           constants.ORDER_TYPE_LIMIT,
		TIF:            constants.TIF_GTC,
		Quantity:       10,
		Price:          95,
		TriggerPrice:   90,
		CloseOnTrigger: true,
	}

	if _, err := s.PlaceOrder(context.Background(), input); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	s.OnPriceTick("BTCUSDT", 90)

	if clearing.lastQty != 3 {
		t.Fatalf("expected clamped qty 3, got %d", clearing.lastQty)
	}
}

func TestReduceOnly_NeverIncreasesPosition(t *testing.T) {
	s, portfolio := newTestService()

	portfolio.addPosition(1, "BTCUSDT", 2, constants.SIDE_LONG)
	input := &types.OrderInput{
		UserID:     1,
		Symbol:     "BTCUSDT",
		Category:   constants.CATEGORY_LINEAR,
		Side:       constants.ORDER_SIDE_BUY,
		Type:       constants.ORDER_TYPE_LIMIT,
		TIF:        constants.TIF_GTC,
		Quantity:   1,
		Price:      100,
		ReduceOnly: true,
	}
	if err := s.validateOrder(input); err != ErrReduceOnlySide {
		t.Fatalf("expected ErrReduceOnlySide for long+buy, got %v", err)
	}

	portfolio.addPosition(1, "BTCUSDT", -2, constants.SIDE_SHORT)
	input.Side = constants.ORDER_SIDE_SELL
	if err := s.validateOrder(input); err != ErrReduceOnlySide {
		t.Fatalf("expected ErrReduceOnlySide for short+sell, got %v", err)
	}
}
