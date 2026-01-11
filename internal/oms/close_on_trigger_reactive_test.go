package oms

import (
	"context"
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func TestCloseOnTrigger_ReactiveClampOnPositionShrink(t *testing.T) {
	s, portfolio := newTestService()
	portfolio.addPosition(1, "BTCUSDT", 5, constants.SIDE_LONG)

	input := &types.OrderInput{
		UserID:         1,
		Symbol:         "BTCUSDT",
		Category:       constants.CATEGORY_LINEAR,
		Side:           constants.ORDER_SIDE_SELL,
		Type:           constants.ORDER_TYPE_LIMIT,
		TIF:            constants.TIF_GTC,
		Quantity:       4,
		Price:          100,
		TriggerPrice:   90,
		CloseOnTrigger: true,
	}

	result, err := s.PlaceOrder(context.Background(), input)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	order := result.Orders[0]

	s.OnPositionUpdate(1, "BTCUSDT", 2, constants.SIDE_LONG)

	if order.Quantity != 2 {
		t.Fatalf("expected clamped quantity 2, got %d", order.Quantity)
	}
}
