package validation

import (
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/position"
	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func TestValidateOrderInput(t *testing.T) {
	tests := []struct {
		name     string
		input    *types.OrderInput
		category int8
		wantErr  bool
	}{
		{
			name: "valid limit order",
			input: &types.OrderInput{
				UserID:   1,
				Symbol:   1,
				Side:     constants.ORDER_SIDE_BUY,
				Type:     constants.ORDER_TYPE_LIMIT,
				Quantity: 10,
				Price:    100,
				TIF:      constants.TIF_GTC,
			},
			category: constants.CATEGORY_SPOT,
			wantErr:  false,
		},
		{
			name: "invalid quantity",
			input: &types.OrderInput{
				UserID:   1,
				Symbol:   1,
				Side:     constants.ORDER_SIDE_BUY,
				Type:     constants.ORDER_TYPE_LIMIT,
				Quantity: 0,
				Price:    100,
			},
			category: constants.CATEGORY_SPOT,
			wantErr:  true,
		},
		{
			name: "invalid price for limit",
			input: &types.OrderInput{
				UserID:   1,
				Symbol:   1,
				Side:     constants.ORDER_SIDE_BUY,
				Type:     constants.ORDER_TYPE_LIMIT,
				Quantity: 10,
				Price:    0,
			},
			category: constants.CATEGORY_SPOT,
			wantErr:  true,
		},
		{
			name: "invalid side",
			input: &types.OrderInput{
				UserID:   1,
				Symbol:   1,
				Side:     99,
				Type:     constants.ORDER_TYPE_LIMIT,
				Quantity: 10,
				Price:    100,
			},
			category: constants.CATEGORY_SPOT,
			wantErr:  true,
		},
		{
			name: "reduceOnly in SPOT",
			input: &types.OrderInput{
				UserID:     1,
				Symbol:     1,
				Side:       constants.ORDER_SIDE_SELL,
				Type:       constants.ORDER_TYPE_LIMIT,
				Quantity:   10,
				Price:      100,
				ReduceOnly: true,
			},
			category: constants.CATEGORY_SPOT,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateOrderInput(tt.input, tt.category)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateOrderInput() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateReduceOnly(t *testing.T) {
	s := state.New()
	userID := types.UserID(1)
	symbol := types.SymbolID(1)

	input := &types.OrderInput{
		UserID:     userID,
		Symbol:     symbol,
		Side:       constants.ORDER_SIDE_SELL,
		Quantity:   5,
		ReduceOnly: true,
	}

	err := ValidateReduceOnly(s, input, constants.CATEGORY_LINEAR)
	if err == nil {
		t.Error("expected error with no position")
	}

	position.UpdatePosition(s, userID, symbol, 10, 100, constants.ORDER_SIDE_BUY)

	err = ValidateReduceOnly(s, input, constants.CATEGORY_LINEAR)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	input.Quantity = 15
	err = ValidateReduceOnly(s, input, constants.CATEGORY_LINEAR)
	if err == nil {
		t.Error("expected error when qty exceeds position")
	}
}

func TestValidateBalance(t *testing.T) {
	s := state.New()
	userID := types.UserID(1)

	input := &types.OrderInput{
		UserID:   userID,
		Symbol:   1,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		Quantity: 10,
		Price:    100,
		TIF:      constants.TIF_GTC,
	}

	us := s.GetUserState(userID)
	us.Balances["USDT"] = &types.UserBalance{
		UserID:    userID,
		Asset:     "USDT",
		Available: 1000,
	}

	err := ValidateBalance(s, input, constants.CATEGORY_SPOT, 100)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	input.TIF = constants.TIF_IOC
	err = ValidateBalance(s, input, constants.CATEGORY_SPOT, 100)
	if err != nil {
		t.Errorf("expected no error for IOC, got %v", err)
	}

	us.Balances["USDT"].Available = 500
	input.TIF = constants.TIF_GTC
	input.Side = constants.ORDER_SIDE_BUY
	err = ValidateBalance(s, input, constants.CATEGORY_SPOT, 100)
	if err == nil {
		t.Error("expected error for insufficient balance")
	}

	us.Balances["USDT"].Available = 1000
	err = ValidateBalance(s, input, constants.CATEGORY_SPOT, 100)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestValidateMargin(t *testing.T) {
	s := state.New()
	userID := types.UserID(1)

	input := &types.OrderInput{
		UserID:   userID,
		Symbol:   1,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		Quantity: 10,
		Price:    100,
	}

	us := s.GetUserState(userID)
	us.Balances["USDT"] = &types.UserBalance{
		UserID: userID,
		Asset:  "USDT",
		Margin: 100,
	}

	err := ValidateMargin(s, input, constants.CATEGORY_LINEAR, 100, 10)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	us.Balances["USDT"].Margin = 50
	err = ValidateMargin(s, input, constants.CATEGORY_LINEAR, 100, 10)
	if err == nil {
		t.Error("expected error for insufficient margin")
	}

	us.Balances["USDT"].Margin = 100
	err = ValidateMargin(s, input, constants.CATEGORY_LINEAR, 100, 10)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}
