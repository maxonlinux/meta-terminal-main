package oms_test

import (
	"context"
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func TestOCODeactivatesSiblingOnTrigger(t *testing.T) {
	s, port := newService()
	setPosition(port, 1, "BTCUSDT", 1, constants.SIDE_LONG, 50000, 10)

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
	sl := result.Orders[1]

	s.OnPriceTick("BTCUSDT", 60000)

	orders := s.GetOrders(1)
	if findOrder(orders, sl.ID) != nil {
		t.Fatalf("expected SL removed from memory after deactivation")
	}
}

func TestOCODeactivatesOnPositionClose(t *testing.T) {
	s, port := newService()
	setPosition(port, 1, "BTCUSDT", 1, constants.SIDE_LONG, 50000, 10)

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

	orders := s.GetOrders(1)
	if findOrder(orders, tp.ID) != nil || findOrder(orders, sl.ID) != nil {
		t.Fatalf("expected both orders removed from memory after deactivation")
	}
}

func TestCloseOnTriggerDeactivatesOnPositionClose(t *testing.T) {
	s, port := newService()
	setPosition(port, 1, "BTCUSDT", 1, constants.SIDE_LONG, 50000, 10)

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

	orders := s.GetOrders(1)
	if findOrder(orders, order.ID) != nil {
		t.Fatalf("expected close-on-trigger order removed from memory after deactivation")
	}
}

func findOrder(orders []*types.Order, id types.OrderID) *types.Order {
	for _, order := range orders {
		if order.ID == id {
			return order
		}
	}
	return nil
}
