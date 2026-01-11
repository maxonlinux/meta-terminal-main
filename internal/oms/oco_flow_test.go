package oms

import (
	"context"
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func TestOCO_DeactivatesSiblingOnTrigger(t *testing.T) {
	s, portfolio := newTestService()
	portfolio.addPosition(1, "BTCUSDT", 1, constants.SIDE_LONG)

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 0,
		Price:    0,
		OCO: &types.OCOInput{
			Quantity: 0,
			TakeProfit: types.OCOChildOrder{
				TriggerPrice: 60000,
				Price:        59900,
				ReduceOnly:   true,
			},
			StopLoss: types.OCOChildOrder{
				TriggerPrice: 40000,
				Price:        40100,
				ReduceOnly:   true,
			},
		},
	}

	result, err := s.PlaceOrder(context.Background(), input)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Orders) != 2 {
		t.Fatalf("expected 2 orders, got %d", len(result.Orders))
	}
	tp := result.Orders[0]
	sl := result.Orders[1]

	s.OnPriceTick("BTCUSDT", 60000)

	var slStatus int8
	for _, order := range s.orders[tp.UserID] {
		if order.ID == sl.ID {
			slStatus = order.Status
			break
		}
	}
	if slStatus != constants.ORDER_STATUS_DEACTIVATED {
		t.Fatalf("expected SL deactivated, got %d", slStatus)
	}
}

func TestOCO_DeactivatesOnPositionClose(t *testing.T) {
	s, portfolio := newTestService()
	portfolio.addPosition(1, "BTCUSDT", 1, constants.SIDE_LONG)

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 0,
		Price:    0,
		OCO: &types.OCOInput{
			Quantity: 0,
			TakeProfit: types.OCOChildOrder{
				TriggerPrice: 60000,
				Price:        59900,
				ReduceOnly:   true,
			},
			StopLoss: types.OCOChildOrder{
				TriggerPrice: 40000,
				Price:        40100,
				ReduceOnly:   true,
			},
		},
	}

	result, err := s.PlaceOrder(context.Background(), input)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Orders) != 2 {
		t.Fatalf("expected 2 orders, got %d", len(result.Orders))
	}
	tp := result.Orders[0]
	sl := result.Orders[1]

	s.OnPositionUpdate(1, "BTCUSDT", 0, constants.SIDE_NONE)

	var tpStatus, slStatus int8
	if order := s.orders[1][tp.ID]; order != nil {
		tpStatus = order.Status
	}
	if order := s.orders[1][sl.ID]; order != nil {
		slStatus = order.Status
	}
	if tpStatus != constants.ORDER_STATUS_DEACTIVATED || slStatus != constants.ORDER_STATUS_DEACTIVATED {
		t.Fatalf("expected both deactivated, got tp=%d sl=%d", tpStatus, slStatus)
	}
}

func TestCloseOnTrigger_DeactivatesOnPositionClose(t *testing.T) {
	s, portfolio := newTestService()
	portfolio.addPosition(1, "BTCUSDT", 1, constants.SIDE_LONG)

	input := &types.OrderInput{
		UserID:         1,
		Symbol:         "BTCUSDT",
		Category:       constants.CATEGORY_LINEAR,
		Side:           constants.ORDER_SIDE_SELL,
		Type:           constants.ORDER_TYPE_LIMIT,
		TIF:            constants.TIF_GTC,
		Quantity:       0,
		Price:          50000,
		TriggerPrice:   49000,
		CloseOnTrigger: true,
	}

	result, err := s.PlaceOrder(context.Background(), input)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Orders) != 1 {
		t.Fatalf("expected 1 order, got %d", len(result.Orders))
	}
	order := result.Orders[0]

	s.OnPositionUpdate(1, "BTCUSDT", 0, constants.SIDE_NONE)

	if got := s.orders[1][order.ID]; got == nil || got.Status != constants.ORDER_STATUS_DEACTIVATED {
		if got == nil {
			t.Fatalf("expected order to remain tracked as deactivated")
		}
		t.Fatalf("expected deactivated, got %d", got.Status)
	}
}
