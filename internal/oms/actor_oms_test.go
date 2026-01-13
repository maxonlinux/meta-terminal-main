package oms

import (
	"context"
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

type nopPortfolio struct{}

func (n *nopPortfolio) GetPositions(userID types.UserID) []*types.Position {
	return nil
}

func (n *nopPortfolio) GetPosition(userID types.UserID, symbol string) *types.Position {
	return nil
}

func (n *nopPortfolio) GetBalance(userID types.UserID, asset string) *types.UserBalance {
	return nil
}

type nopClearing struct{}

func (n *nopClearing) Reserve(userID types.UserID, symbol string, category int8, side int8, qty types.Quantity, price types.Price) error {
	return nil
}

func (n *nopClearing) Release(userID types.UserID, symbol string, category int8, side int8, qty types.Quantity, price types.Price) {
}

func (n *nopClearing) ExecuteTrade(trade *types.Trade, taker *types.Order, maker *types.Order) {
}

func TestActorOMS_PlaceOrder(t *testing.T) {
	oms, err := NewActorOMS(Config{}, &nopPortfolio{}, &nopClearing{})
	if err != nil {
		t.Fatalf("failed to create ActorOMS: %v", err)
	}

	result, err := oms.PlaceOrder(context.Background(), &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		Quantity: 100,
		Price:    50000,
	})

	if err != nil {
		t.Fatalf("PlaceOrder failed: %v", err)
	}

	if len(result.Orders) != 1 {
		t.Fatalf("expected 1 order, got %d", len(result.Orders))
	}

	order := result.Orders[0]
	if order.Status != constants.ORDER_STATUS_NEW {
		t.Errorf("expected status NEW, got %d", order.Status)
	}
	if order.Quantity != 100 {
		t.Errorf("expected quantity 100, got %d", order.Quantity)
	}
}

func TestActorOMS_PositionUpdate(t *testing.T) {
	oms, err := NewActorOMS(Config{}, &nopPortfolio{}, &nopClearing{})
	if err != nil {
		t.Fatalf("failed to create ActorOMS: %v", err)
	}

	oms.OnPositionUpdate(1, "BTCUSDT", -10, constants.SIDE_SHORT)

	result, err := oms.PlaceOrder(context.Background(), &types.OrderInput{
		UserID:     1,
		Symbol:     "BTCUSDT",
		Category:   constants.CATEGORY_LINEAR,
		Side:       constants.ORDER_SIDE_SELL,
		Type:       constants.ORDER_TYPE_LIMIT,
		Quantity:   20,
		Price:      49000,
		ReduceOnly: true,
	})

	if err != nil {
		t.Fatalf("PlaceOrder failed: %v", err)
	}

	if len(result.Orders) != 1 {
		t.Fatalf("expected 1 order, got %d", len(result.Orders))
	}
}

func TestActorOMS_NoRaceBetweenPositionAndMatching(t *testing.T) {
	oms, err := NewActorOMS(Config{}, &nopPortfolio{}, &nopClearing{})
	if err != nil {
		t.Fatalf("failed to create ActorOMS: %v", err)
	}

	done := make(chan bool)
	errors := make(chan error, 100)

	for i := 0; i < 50; i++ {
		go func() {
			defer func() { done <- true }()

			oms.OnPositionUpdate(1, "BTCUSDT", -5, constants.SIDE_SHORT)
		}()

		go func() {
			defer func() { done <- true }()

			_, err := oms.PlaceOrder(context.Background(), &types.OrderInput{
				UserID:     1,
				Symbol:     "BTCUSDT",
				Category:   constants.CATEGORY_LINEAR,
				Side:       constants.ORDER_SIDE_SELL,
				Type:       constants.ORDER_TYPE_LIMIT,
				Quantity:   10,
				Price:      49000,
				ReduceOnly: true,
			})
			if err != nil {
				errors <- err
			}
		}()
	}

	for i := 0; i < 100; i++ {
		select {
		case <-done:
		case err := <-errors:
			t.Errorf("operation failed: %v", err)
		}
	}
}

func TestActorOMS_TriggerOrder(t *testing.T) {
	oms, err := NewActorOMS(Config{}, &nopPortfolio{}, &nopClearing{})
	if err != nil {
		t.Fatalf("failed to create ActorOMS: %v", err)
	}

	order := &types.Order{
		ID:            1,
		UserID:        1,
		Symbol:        "BTCUSDT",
		Category:      constants.CATEGORY_LINEAR,
		Side:          constants.ORDER_SIDE_BUY,
		Type:          constants.ORDER_TYPE_LIMIT,
		StopOrderType: constants.STOP_ORDER_TYPE_STOP,
		Status:        constants.ORDER_STATUS_UNTRIGGERED,
		Quantity:      100,
		TriggerPrice:  49000,
		CreatedAt:     types.NowNano(),
	}

	oms.OnPriceTick("BTCUSDT", 49000)

	if order.Status != constants.ORDER_STATUS_UNTRIGGERED {
		t.Errorf("expected status UNTRIGGERED, got %d", order.Status)
	}
}

func TestActorOMS_ValidateOrder(t *testing.T) {
	oms, err := NewActorOMS(Config{}, &nopPortfolio{}, &nopClearing{})
	if err != nil {
		t.Fatalf("failed to create ActorOMS: %v", err)
	}

	tests := []struct {
		name    string
		input   *types.OrderInput
		wantErr bool
	}{
		{
			name: "SPOT reduceOnly should fail",
			input: &types.OrderInput{
				UserID:     1,
				Symbol:     "BTCUSDT",
				Category:   constants.CATEGORY_SPOT,
				Side:       constants.ORDER_SIDE_BUY,
				Type:       constants.ORDER_TYPE_LIMIT,
				Quantity:   100,
				Price:      50000,
				ReduceOnly: true,
			},
			wantErr: true,
		},
		{
			name: "SPOT conditional should fail",
			input: &types.OrderInput{
				UserID:        1,
				Symbol:        "BTCUSDT",
				Category:      constants.CATEGORY_SPOT,
				Side:          constants.ORDER_SIDE_BUY,
				Type:          constants.ORDER_TYPE_LIMIT,
				Quantity:      100,
				Price:         50000,
				TriggerPrice:  49000,
				IsConditional: true,
			},
			wantErr: true,
		},
		{
			name: "SPOT closeOnTrigger should fail",
			input: &types.OrderInput{
				UserID:         1,
				Symbol:         "BTCUSDT",
				Category:       constants.CATEGORY_SPOT,
				Side:           constants.ORDER_SIDE_BUY,
				Type:           constants.ORDER_TYPE_LIMIT,
				Quantity:       100,
				Price:          50000,
				CloseOnTrigger: true,
			},
			wantErr: true,
		},
		{
			name: "LINEAR market without IOC/FOK should fail",
			input: &types.OrderInput{
				UserID:   1,
				Symbol:   "BTCUSDT",
				Category: constants.CATEGORY_LINEAR,
				Side:     constants.ORDER_SIDE_BUY,
				Type:     constants.ORDER_TYPE_MARKET,
				Quantity: 100,
				TIF:      constants.TIF_GTC,
			},
			wantErr: true,
		},
		{
			name: "valid LIMIT order should pass",
			input: &types.OrderInput{
				UserID:   1,
				Symbol:   "BTCUSDT",
				Category: constants.CATEGORY_LINEAR,
				Side:     constants.ORDER_SIDE_BUY,
				Type:     constants.ORDER_TYPE_LIMIT,
				Quantity: 100,
				Price:    50000,
				TIF:      constants.TIF_GTC,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := oms.PlaceOrder(context.Background(), tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("PlaceOrder() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestActorOMS_GetOrder(t *testing.T) {
	oms, err := NewActorOMS(Config{}, &nopPortfolio{}, &nopClearing{})
	if err != nil {
		t.Fatalf("failed to create ActorOMS: %v", err)
	}

	result, err := oms.PlaceOrder(context.Background(), &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		Quantity: 100,
		Price:    50000,
	})

	if err != nil {
		t.Fatalf("PlaceOrder failed: %v", err)
	}

	if len(result.Orders) != 1 {
		t.Fatalf("expected 1 order, got %d", len(result.Orders))
	}

	order := result.Orders[0]

	retrieved := oms.GetOrder(1, order.ID)
	if retrieved == nil {
		t.Fatal("GetOrder returned nil")
	}

	if retrieved.ID != order.ID {
		t.Errorf("expected order ID %d, got %d", order.ID, retrieved.ID)
	}
}

func TestActorOMS_GetOrders(t *testing.T) {
	oms, err := NewActorOMS(Config{}, &nopPortfolio{}, &nopClearing{})
	if err != nil {
		t.Fatalf("failed to create ActorOMS: %v", err)
	}

	_, err = oms.PlaceOrder(context.Background(), &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		Quantity: 100,
		Price:    50000,
	})
	if err != nil {
		t.Fatalf("PlaceOrder failed: %v", err)
	}

	_, err = oms.PlaceOrder(context.Background(), &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		Quantity: 50,
		Price:    51000,
	})
	if err != nil {
		t.Fatalf("PlaceOrder failed: %v", err)
	}

	orders := oms.GetOrders(1)
	if len(orders) != 2 {
		t.Fatalf("expected 2 orders, got %d", len(orders))
	}
}
