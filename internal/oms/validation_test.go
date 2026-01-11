package oms

import (
	"context"
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/orderbook"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func TestValidateOrder_CloseOnTriggerNoPosition(t *testing.T) {
	s, _ := newTestService()

	input := &types.OrderInput{
		UserID:         1,
		Symbol:         "BTCUSDT",
		Category:       constants.CATEGORY_LINEAR,
		Side:           constants.ORDER_SIDE_BUY,
		Type:           constants.ORDER_TYPE_LIMIT,
		TIF:            constants.TIF_GTC,
		Quantity:       1,
		Price:          50000,
		CloseOnTrigger: true,
	}

	err := s.validateOrder(input)
	if err != ErrCloseOnTriggerNoPosition {
		t.Errorf("Expected ErrCloseOnTriggerNoPosition, got %v", err)
	}
}

func TestValidateOrder_CloseOnTriggerWithPosition(t *testing.T) {
	s, portfolio := newTestService()
	portfolio.addPosition(1, "BTCUSDT", 1, constants.SIDE_LONG)

	input := &types.OrderInput{
		UserID:         1,
		Symbol:         "BTCUSDT",
		Category:       constants.CATEGORY_LINEAR,
		Side:           constants.ORDER_SIDE_SELL,
		Type:           constants.ORDER_TYPE_LIMIT,
		TIF:            constants.TIF_GTC,
		Quantity:       1,
		Price:          45000,
		TriggerPrice:   44000,
		CloseOnTrigger: true,
	}

	err := s.validateOrder(input)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestIsConditionalFlag(t *testing.T) {
	s, _ := newTestService()

	regularOrder := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 1,
		Price:    50000,
	}

	conditionalOrder := &types.OrderInput{
		UserID:       1,
		Symbol:       "BTCUSDT",
		Category:     constants.CATEGORY_LINEAR,
		Side:         constants.ORDER_SIDE_BUY,
		Type:         constants.ORDER_TYPE_LIMIT,
		TIF:          constants.TIF_GTC,
		Quantity:     1,
		Price:        50000,
		TriggerPrice: 48000,
	}

	if regularOrder.IsConditional {
		t.Error("Expected IsConditional=false for regular order")
	}

	_ = s.validateOrder(conditionalOrder)

	if !conditionalOrder.IsConditional {
		t.Error("Expected IsConditional=true after validation")
	}
}

func TestSelfMatchPrevention_DifferentUsers(t *testing.T) {
	s, _ := newTestService()

	ob := orderbook.New()
	s.orderbooks[constants.CATEGORY_LINEAR]["BTCUSDT"] = ob

	otherOrder := &types.Order{
		ID:       100,
		UserID:   2,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Status:   constants.ORDER_STATUS_NEW,
		Price:    50000,
		Quantity: 1,
	}
	ob.Add(otherOrder)

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 1,
		Price:    50000,
	}

	err := s.checkSelfMatch(input)
	if err != nil {
		t.Errorf("Expected no error for different users, got %v", err)
	}
}

func TestSelfMatchPrevention_SameUser(t *testing.T) {
	s, _ := newTestService()

	ob := orderbook.New()
	s.orderbooks[constants.CATEGORY_LINEAR]["BTCUSDT"] = ob

	selfOrder := &types.Order{
		ID:       100,
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Status:   constants.ORDER_STATUS_NEW,
		Price:    50000,
		Quantity: 1,
	}
	ob.Add(selfOrder)
	s.orders[1] = map[types.OrderID]*types.Order{100: selfOrder}

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 1,
		Price:    50000,
	}

	err := s.checkSelfMatch(input)
	if err != ErrSelfMatch {
		t.Errorf("Expected ErrSelfMatch, got %v", err)
	}
}

func TestPlaceOrder_ConditionalOrderFlag(t *testing.T) {
	s, _ := newTestService()

	input := &types.OrderInput{
		UserID:       1,
		Symbol:       "BTCUSDT",
		Category:     constants.CATEGORY_LINEAR,
		Side:         constants.ORDER_SIDE_BUY,
		Type:         constants.ORDER_TYPE_LIMIT,
		TIF:          constants.TIF_GTC,
		Quantity:     1,
		Price:        50000,
		TriggerPrice: 48000,
	}

	_ = s.validateOrder(input)

	if !input.IsConditional {
		t.Error("Expected IsConditional=true after validation")
	}

	result, err := s.PlaceOrder(context.Background(), input)
	if err != nil {
		t.Fatalf("PlaceOrder failed: %v", err)
	}

	if len(result.Orders) != 1 {
		t.Fatalf("Expected 1 order, got %d", len(result.Orders))
	}

	if !result.Orders[0].IsConditional {
		t.Error("Expected order.IsConditional=true")
	}

	if result.Orders[0].Status != constants.ORDER_STATUS_UNTRIGGERED {
		t.Errorf("Expected UNTRIGGERED status for conditional order, got %d", result.Orders[0].Status)
	}
}

func TestOCO_QuantityZeroMeansFullPositionClose(t *testing.T) {
	s, portfolio := newTestService()
	portfolio.addPosition(1, "BTCUSDT", 2, constants.SIDE_LONG)

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Quantity: 2,
		OCO: &types.OCOInput{
			Quantity: 0,
			TakeProfit: types.OCOChildOrder{
				TriggerPrice: 55000,
				Price:        54900,
				ReduceOnly:   true,
			},
			StopLoss: types.OCOChildOrder{
				TriggerPrice: 45000,
				Price:        45100,
				ReduceOnly:   true,
			},
		},
	}

	result, err := s.PlaceOrder(context.Background(), input)
	if err != nil {
		t.Fatalf("OCO PlaceOrder failed: %v", err)
	}

	if len(result.Orders) != 2 {
		t.Fatalf("Expected 2 orders for OCO, got %d", len(result.Orders))
	}

	for i, order := range result.Orders {
		if order.Quantity != 0 {
			t.Errorf("OCO order %d: expected Quantity=0, got %d", i+1, order.Quantity)
		}
		if !order.CloseOnTrigger {
			t.Errorf("OCO order %d: expected CloseOnTrigger=true", i+1)
		}
	}

	if result.Remaining != 2 {
		t.Errorf("Expected Remaining=2 (position size), got %d", result.Remaining)
	}
}

func TestReduceOnly_QuantityZeroMeansFullPosition(t *testing.T) {
	s, portfolio := newTestService()
	portfolio.addPosition(1, "ETHUSDT", 3, constants.SIDE_SHORT)

	input := &types.OrderInput{
		UserID:       1,
		Symbol:       "ETHUSDT",
		Category:     constants.CATEGORY_LINEAR,
		Side:         constants.ORDER_SIDE_BUY,
		Type:         constants.ORDER_TYPE_LIMIT,
		TIF:          constants.TIF_GTC,
		Quantity:     0,
		Price:        3000,
		ReduceOnly:   true,
		TriggerPrice: 28000,
	}

	_ = s.validateOrder(input)

	result, err := s.PlaceOrder(context.Background(), input)
	if err != nil {
		t.Fatalf("PlaceOrder failed: %v", err)
	}

	if len(result.Orders) != 1 {
		t.Fatalf("Expected 1 order, got %d", len(result.Orders))
	}

	if result.Orders[0].Quantity != 0 {
		t.Errorf("Expected Quantity=0 for reduceOnly, got %d", result.Orders[0].Quantity)
	}
}

func TestCloseOnTrigger_QuantityZeroFullClose(t *testing.T) {
	s, portfolio := newTestService()
	portfolio.addPosition(1, "BTCUSDT", 5, constants.SIDE_LONG)

	input := &types.OrderInput{
		UserID:         1,
		Symbol:         "BTCUSDT",
		Category:       constants.CATEGORY_LINEAR,
		Side:           constants.ORDER_SIDE_SELL,
		Type:           constants.ORDER_TYPE_LIMIT,
		TIF:            constants.TIF_GTC,
		Quantity:       0,
		Price:          45000,
		TriggerPrice:   46000,
		CloseOnTrigger: true,
	}

	_ = s.validateOrder(input)

	result, err := s.PlaceOrder(context.Background(), input)
	if err != nil {
		t.Fatalf("PlaceOrder failed: %v", err)
	}

	if len(result.Orders) != 1 {
		t.Fatalf("Expected 1 order, got %d", len(result.Orders))
	}

	if result.Orders[0].Quantity != 0 {
		t.Errorf("Expected Quantity=0 for closeOnTrigger, got %d", result.Orders[0].Quantity)
	}
}

func BenchmarkPlaceOrder_Conditional(b *testing.B) {
	b.ReportAllocs()
	s, portfolio := newTestService()
	portfolio.addPosition(1, "BTCUSDT", 10, constants.SIDE_LONG)

	input := &types.OrderInput{
		UserID:         1,
		Symbol:         "BTCUSDT",
		Category:       constants.CATEGORY_LINEAR,
		Side:           constants.ORDER_SIDE_SELL,
		Type:           constants.ORDER_TYPE_LIMIT,
		TIF:            constants.TIF_GTC,
		Quantity:       1,
		Price:          45000,
		TriggerPrice:   46000,
		ReduceOnly:     true,
		CloseOnTrigger: true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, _ := s.PlaceOrder(context.Background(), input)
		if result != nil && len(result.Orders) == 1 {
			s.cancelOrder(result.Orders[0])
		}
	}
}

func BenchmarkSelfMatchCheck(b *testing.B) {
	b.ReportAllocs()
	s, _ := newTestService()

	ob := orderbook.New()
	s.orderbooks[constants.CATEGORY_LINEAR]["BTCUSDT"] = ob

	for i := 0; i < 100; i++ {
		order := &types.Order{
			ID:       types.OrderID(i),
			UserID:   types.UserID(i % 10),
			Symbol:   "BTCUSDT",
			Category: constants.CATEGORY_LINEAR,
			Side:     constants.ORDER_SIDE_SELL,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Status:   constants.ORDER_STATUS_NEW,
			Price:    types.Price(50000 + i),
			Quantity: 1,
		}
		ob.Add(order)
		if s.orders[order.UserID] == nil {
			s.orders[order.UserID] = make(map[types.OrderID]*types.Order)
		}
		s.orders[order.UserID][order.ID] = order
	}

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 1,
		Price:    50500,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = s.checkSelfMatch(input)
	}
}

func TestValidateOrder_QuantityZeroForRegularOrder(t *testing.T) {
	s, _ := newTestService()

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 0, // Regular order with qty=0 should fail
		Price:    50000,
	}

	err := s.validateOrder(input)
	if err != ErrInvalidQuantity {
		t.Errorf("Expected ErrInvalidQuantity for regular order with qty=0, got %v", err)
	}
}

func TestValidateOrder_QuantityZeroForConditionalOrder(t *testing.T) {
	s, _ := newTestService()

	input := &types.OrderInput{
		UserID:       1,
		Symbol:       "BTCUSDT",
		Category:     constants.CATEGORY_LINEAR,
		Side:         constants.ORDER_SIDE_BUY,
		Type:         constants.ORDER_TYPE_LIMIT,
		TIF:          constants.TIF_GTC,
		Quantity:     0,
		Price:        50000,
		TriggerPrice: 48000, // Conditional order
	}

	err := s.validateOrder(input)
	if err != nil {
		t.Errorf("Expected no error for conditional order with qty=0, got %v", err)
	}
}

func TestValidateOrder_QuantityZeroForCloseOnTrigger(t *testing.T) {
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
		Price:          45000,
		TriggerPrice:   44000,
		CloseOnTrigger: true,
	}

	err := s.validateOrder(input)
	if err != nil {
		t.Errorf("Expected no error for closeOnTrigger order with qty=0, got %v", err)
	}
}

func TestValidateOrder_InvalidSymbol(t *testing.T) {
	s, _ := newTestService()

	testCases := []struct {
		name   string
		symbol string
	}{
		{"empty symbol", ""},
		{"too short", "A"},
		{"too long", "ABCDEFGHIJKLMNOPQRSTUVWXYZ123"},
		{"invalid characters", "BTC@USDT"},
		{"no quote asset", "BTC"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			input := &types.OrderInput{
				UserID:   1,
				Symbol:   tc.symbol,
				Category: constants.CATEGORY_LINEAR,
				Side:     constants.ORDER_SIDE_BUY,
				Type:     constants.ORDER_TYPE_LIMIT,
				TIF:      constants.TIF_GTC,
				Quantity: 1,
				Price:    50000,
			}

			err := s.validateOrder(input)
			if err != ErrInvalidSymbol {
				t.Errorf("Expected ErrInvalidSymbol, got %v", err)
			}
		})
	}
}

func TestValidateOrder_ValidSymbols(t *testing.T) {
	s, _ := newTestService()

	validSymbols := []string{"BTCUSDT", "ETHUSDT", "SOLUSDC", "DOGEUSD", "BTCUSD"}

	for _, symbol := range validSymbols {
		t.Run(symbol, func(t *testing.T) {
			input := &types.OrderInput{
				UserID:   1,
				Symbol:   symbol,
				Category: constants.CATEGORY_LINEAR,
				Side:     constants.ORDER_SIDE_BUY,
				Type:     constants.ORDER_TYPE_LIMIT,
				TIF:      constants.TIF_GTC,
				Quantity: 1,
				Price:    50000,
			}

			err := s.validateOrder(input)
			if err != nil {
				t.Errorf("Expected no error for valid symbol %s, got %v", symbol, err)
			}
		})
	}
}

func TestValidateOrder_InvalidPriceForLimit(t *testing.T) {
	s, _ := newTestService()

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 1,
		Price:    -100, // Negative price for LIMIT
	}

	err := s.validateOrder(input)
	if err != ErrInvalidPrice {
		t.Errorf("Expected ErrInvalidPrice for LIMIT with negative price, got %v", err)
	}
}

func TestValidateOrder_InvalidCategory(t *testing.T) {
	s, _ := newTestService()

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: 99, // Invalid category
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 1,
		Price:    50000,
	}

	err := s.validateOrder(input)
	if err != ErrInvalidCategory {
		t.Errorf("Expected ErrInvalidCategory, got %v", err)
	}
}

func TestValidateOrder_InvalidSide(t *testing.T) {
	s, _ := newTestService()

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     99, // Invalid side
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 1,
		Price:    50000,
	}

	err := s.validateOrder(input)
	if err != ErrInvalidSide {
		t.Errorf("Expected ErrInvalidSide, got %v", err)
	}
}

func TestValidateOrder_InvalidOrderType(t *testing.T) {
	s, _ := newTestService()

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     99, // Invalid order type
		TIF:      constants.TIF_GTC,
		Quantity: 1,
		Price:    50000,
	}

	err := s.validateOrder(input)
	if err != ErrInvalidOrderType {
		t.Errorf("Expected ErrInvalidOrderType, got %v", err)
	}
}

func TestValidateOrder_InvalidTIF(t *testing.T) {
	s, _ := newTestService()

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      99, // Invalid TIF
		Quantity: 1,
		Price:    50000,
	}

	err := s.validateOrder(input)
	if err != ErrInvalidTIF {
		t.Errorf("Expected ErrInvalidTIF, got %v", err)
	}
}

func TestOCOValidation_LongPositionTPNotGreaterThanSL(t *testing.T) {
	s, portfolio := newTestService()
	portfolio.addPosition(1, "BTCUSDT", 2, constants.SIDE_LONG) // LONG position

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Quantity: 2,
		OCO: &types.OCOInput{
			Quantity: 1,
			TakeProfit: types.OCOChildOrder{
				TriggerPrice: 50000, // TP = 50000
				Price:        49900,
				ReduceOnly:   true,
			},
			StopLoss: types.OCOChildOrder{
				TriggerPrice: 51000, // SL = 51000 (TP NOT > SL!)
				Price:        51100,
				ReduceOnly:   true,
			},
		},
	}

	_, err := s.PlaceOrder(context.Background(), input)
	if err != ErrOCOTPTriggerInvalid {
		t.Errorf("Expected ErrOCOTPTriggerInvalid for LONG with TP <= SL, got %v", err)
	}
}

func TestOCOValidation_LongPositionTPGreaterThanSL(t *testing.T) {
	s, portfolio := newTestService()
	portfolio.addPosition(1, "BTCUSDT", 2, constants.SIDE_LONG) // LONG position

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Quantity: 2,
		OCO: &types.OCOInput{
			Quantity: 1,
			TakeProfit: types.OCOChildOrder{
				TriggerPrice: 55000, // TP = 55000
				Price:        54900,
				ReduceOnly:   true,
			},
			StopLoss: types.OCOChildOrder{
				TriggerPrice: 45000, // SL = 45000 (TP > SL ✓)
				Price:        45100,
				ReduceOnly:   true,
			},
		},
	}

	_, err := s.PlaceOrder(context.Background(), input)
	if err != nil {
		t.Errorf("Expected no error for LONG with TP > SL, got %v", err)
	}
}

func TestOCOValidation_ShortPositionTPNotLessThanSL(t *testing.T) {
	s, portfolio := newTestService()
	portfolio.addPosition(1, "BTCUSDT", 2, constants.SIDE_SHORT) // SHORT position

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Quantity: 2,
		OCO: &types.OCOInput{
			Quantity: 1,
			TakeProfit: types.OCOChildOrder{
				TriggerPrice: 40000, // TP = 40000
				Price:        40100,
				ReduceOnly:   true,
			},
			StopLoss: types.OCOChildOrder{
				TriggerPrice: 35000, // SL = 35000 (TP NOT < SL!)
				Price:        34900,
				ReduceOnly:   true,
			},
		},
	}

	_, err := s.PlaceOrder(context.Background(), input)
	if err != ErrOCOSLTriggerInvalid {
		t.Errorf("Expected ErrOCOSLTriggerInvalid for SHORT with TP >= SL, got %v", err)
	}
}

func TestOCOValidation_ShortPositionTPLessThanSL(t *testing.T) {
	s, portfolio := newTestService()
	portfolio.addPosition(1, "BTCUSDT", 2, constants.SIDE_SHORT) // SHORT position

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Quantity: 2,
		OCO: &types.OCOInput{
			Quantity: 1,
			TakeProfit: types.OCOChildOrder{
				TriggerPrice: 45000, // TP = 45000
				Price:        45100,
				ReduceOnly:   true,
			},
			StopLoss: types.OCOChildOrder{
				TriggerPrice: 55000, // SL = 55000 (TP < SL ✓)
				Price:        54900,
				ReduceOnly:   true,
			},
		},
	}

	_, err := s.PlaceOrder(context.Background(), input)
	if err != nil {
		t.Errorf("Expected no error for SHORT with TP < SL, got %v", err)
	}
}

func TestImpossibleCombinations_SPOT_ReduceOnly(t *testing.T) {
	s, _ := newTestService()

	input := &types.OrderInput{
		UserID:     1,
		Symbol:     "BTCUSDT",
		Category:   constants.CATEGORY_SPOT,
		Side:       constants.ORDER_SIDE_SELL,
		Type:       constants.ORDER_TYPE_LIMIT,
		TIF:        constants.TIF_GTC,
		Quantity:   1,
		Price:      50000,
		ReduceOnly: true,
	}

	err := s.validateOrder(input)
	if err != ErrReduceOnlySpot {
		t.Errorf("Expected ErrReduceOnlySpot for SPOT reduceOnly, got %v", err)
	}
}

func TestImpossibleCombinations_SPOT_Conditional(t *testing.T) {
	s, _ := newTestService()

	input := &types.OrderInput{
		UserID:       1,
		Symbol:       "BTCUSDT",
		Category:     constants.CATEGORY_SPOT,
		Side:         constants.ORDER_SIDE_BUY,
		Type:         constants.ORDER_TYPE_LIMIT,
		TIF:          constants.TIF_GTC,
		Quantity:     1,
		Price:        50000,
		TriggerPrice: 49000,
	}

	err := s.validateOrder(input)
	if err != ErrConditionalSpot {
		t.Errorf("Expected ErrConditionalSpot for SPOT conditional, got %v", err)
	}
}

func TestImpossibleCombinations_SPOT_CloseOnTrigger(t *testing.T) {
	s, _ := newTestService()

	input := &types.OrderInput{
		UserID:         1,
		Symbol:         "BTCUSDT",
		Category:       constants.CATEGORY_SPOT,
		Side:           constants.ORDER_SIDE_BUY,
		Type:           constants.ORDER_TYPE_LIMIT,
		TIF:            constants.TIF_GTC,
		Quantity:       1,
		Price:          50000,
		CloseOnTrigger: true,
	}

	err := s.validateOrder(input)
	if err != ErrCloseOnTriggerSpot {
		t.Errorf("Expected ErrCloseOnTriggerSpot for SPOT closeOnTrigger, got %v", err)
	}
}

func TestImpossibleCombinations_SPOT_OCO(t *testing.T) {
	s, portfolio := newTestService()
	portfolio.addPosition(1, "BTCUSDT", 2, constants.SIDE_LONG)

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_SPOT,
		Side:     constants.ORDER_SIDE_BUY,
		Quantity: 2,
		OCO: &types.OCOInput{
			Quantity: 1,
			TakeProfit: types.OCOChildOrder{
				TriggerPrice: 55000,
				Price:        54900,
				ReduceOnly:   true,
			},
			StopLoss: types.OCOChildOrder{
				TriggerPrice: 45000,
				Price:        45100,
				ReduceOnly:   true,
			},
		},
	}

	_, err := s.PlaceOrder(context.Background(), input)
	if err != ErrOCOSpot {
		t.Errorf("Expected ErrOCOSpot for SPOT OCO, got %v", err)
	}
}

func TestImpossibleCombinations_Market_GTC(t *testing.T) {
	s, _ := newTestService()

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_MARKET,
		TIF:      constants.TIF_GTC,
		Quantity: 1,
	}

	err := s.validateOrder(input)
	if err != ErrMarketTIF {
		t.Errorf("Expected ErrMarketTIF for MARKET with GTC, got %v", err)
	}
}

func TestImpossibleCombinations_Market_PostOnly(t *testing.T) {
	s, _ := newTestService()

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_MARKET,
		TIF:      constants.TIF_POST_ONLY,
		Quantity: 1,
	}

	err := s.validateOrder(input)
	if err != ErrMarketTIF {
		t.Errorf("Expected ErrMarketTIF for MARKET with POST_ONLY, got %v", err)
	}
}

func TestImpossibleCombinations_LimitPriceZero(t *testing.T) {
	s, _ := newTestService()

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 1,
		Price:    0,
	}

	err := s.validateOrder(input)
	if err != ErrInvalidPrice {
		t.Errorf("Expected ErrInvalidPrice for LIMIT with zero price, got %v", err)
	}
}

func TestImpossibleCombinations_InvalidCategory(t *testing.T) {
	s, _ := newTestService()

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: 99,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 1,
		Price:    50000,
	}

	err := s.validateOrder(input)
	if err != ErrInvalidCategory {
		t.Errorf("Expected ErrInvalidCategory, got %v", err)
	}
}

func TestImpossibleCombinations_InvalidSide(t *testing.T) {
	s, _ := newTestService()

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     99,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 1,
		Price:    50000,
	}

	err := s.validateOrder(input)
	if err != ErrInvalidSide {
		t.Errorf("Expected ErrInvalidSide, got %v", err)
	}
}

func TestImpossibleCombinations_InvalidOrderType(t *testing.T) {
	s, _ := newTestService()

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     99,
		TIF:      constants.TIF_GTC,
		Quantity: 1,
		Price:    50000,
	}

	err := s.validateOrder(input)
	if err != ErrInvalidOrderType {
		t.Errorf("Expected ErrInvalidOrderType, got %v", err)
	}
}

func TestImpossibleCombinations_InvalidTIF(t *testing.T) {
	s, _ := newTestService()

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      99,
		Quantity: 1,
		Price:    50000,
	}

	err := s.validateOrder(input)
	if err != ErrInvalidTIF {
		t.Errorf("Expected ErrInvalidTIF, got %v", err)
	}
}

type trackingClearing struct {
	reserves []reserveInfo
	releases []releaseInfo
	trades   []*types.Trade
}

type reserveInfo struct {
	userID   types.UserID
	symbol   string
	category int8
	side     int8
	qty      types.Quantity
	price    types.Price
}

type releaseInfo struct {
	userID   types.UserID
	symbol   string
	category int8
	side     int8
	qty      types.Quantity
	price    types.Price
}

func newTrackingClearing() *trackingClearing {
	return &trackingClearing{
		reserves: make([]reserveInfo, 0),
		releases: make([]releaseInfo, 0),
		trades:   make([]*types.Trade, 0),
	}
}

func (tc *trackingClearing) Reserve(userID types.UserID, symbol string, category int8, side int8, qty types.Quantity, price types.Price) error {
	tc.reserves = append(tc.reserves, reserveInfo{userID, symbol, category, side, qty, price})
	return nil
}

func (tc *trackingClearing) Release(userID types.UserID, symbol string, category int8, side int8, qty types.Quantity, price types.Price) {
	tc.releases = append(tc.releases, releaseInfo{userID, symbol, category, side, qty, price})
}

func (tc *trackingClearing) ExecuteTrade(trade *types.Trade, taker *types.Order, maker *types.Order) {
	tc.trades = append(tc.trades, trade)
}

func TestValidateOrder_SpotRestrictions(t *testing.T) {
	s, _ := newTestService()

	input := &types.OrderInput{
		UserID:     1,
		Symbol:     "BTCUSDT",
		Category:   constants.CATEGORY_SPOT,
		Side:       constants.ORDER_SIDE_BUY,
		Type:       constants.ORDER_TYPE_LIMIT,
		TIF:        constants.TIF_GTC,
		Quantity:   1,
		Price:      100,
		ReduceOnly: true,
	}
	if err := s.validateOrder(input); err != ErrReduceOnlySpot {
		t.Fatalf("expected ErrReduceOnlySpot, got %v", err)
	}

	input.ReduceOnly = false
	input.TriggerPrice = 90
	if err := s.validateOrder(input); err != ErrConditionalSpot {
		t.Fatalf("expected ErrConditionalSpot, got %v", err)
	}

	input.TriggerPrice = 0
	input.CloseOnTrigger = true
	if err := s.validateOrder(input); err != ErrCloseOnTriggerSpot {
		t.Fatalf("expected ErrCloseOnTriggerSpot, got %v", err)
	}
}

func TestValidateOrder_LinearMarketTIF(t *testing.T) {
	s, _ := newTestService()

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_MARKET,
		TIF:      constants.TIF_GTC,
		Quantity: 1,
	}

	if err := s.validateOrder(input); err != ErrMarketTIF {
		t.Fatalf("expected ErrMarketTIF, got %v", err)
	}
}

func TestValidateOrder_InvalidFields(t *testing.T) {
	s, _ := newTestService()

	base := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 1,
		Price:    100,
	}

	input := *base
	input.Category = 9
	if err := s.validateOrder(&input); err != ErrInvalidCategory {
		t.Fatalf("expected ErrInvalidCategory, got %v", err)
	}

	input = *base
	input.Side = 9
	if err := s.validateOrder(&input); err != ErrInvalidSide {
		t.Fatalf("expected ErrInvalidSide, got %v", err)
	}

	input = *base
	input.Type = 9
	if err := s.validateOrder(&input); err != ErrInvalidOrderType {
		t.Fatalf("expected ErrInvalidOrderType, got %v", err)
	}

	input = *base
	input.TIF = 9
	if err := s.validateOrder(&input); err != ErrInvalidTIF {
		t.Fatalf("expected ErrInvalidTIF, got %v", err)
	}

	input = *base
	input.StopOrderType = 9
	if err := s.validateOrder(&input); err != ErrInvalidStopOrderType {
		t.Fatalf("expected ErrInvalidStopOrderType, got %v", err)
	}

	input = *base
	input.Symbol = "!"
	if err := s.validateOrder(&input); err != ErrInvalidSymbol {
		t.Fatalf("expected ErrInvalidSymbol, got %v", err)
	}
}

func TestValidateOrder_CloseOnTriggerRequiresTrigger(t *testing.T) {
	s, portfolio := newTestService()
	portfolio.addPosition(1, "BTCUSDT", 1, constants.SIDE_LONG)

	input := &types.OrderInput{
		UserID:         1,
		Symbol:         "BTCUSDT",
		Category:       constants.CATEGORY_LINEAR,
		Side:           constants.ORDER_SIDE_SELL,
		Type:           constants.ORDER_TYPE_LIMIT,
		TIF:            constants.TIF_GTC,
		Quantity:       1,
		Price:          100,
		CloseOnTrigger: true,
		TriggerPrice:   0,
	}
	if err := s.validateOrder(input); err != ErrInvalidTriggerPrice {
		t.Fatalf("expected ErrInvalidTriggerPrice, got %v", err)
	}
}

func TestValidateOrder_PostOnlyWouldCross(t *testing.T) {
	s, _ := newTestService()

	ob := orderbook.New()
	s.orderbooks[constants.CATEGORY_LINEAR]["BTCUSDT"] = ob
	ob.Add(&types.Order{
		ID:       1,
		UserID:   2,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Status:   constants.ORDER_STATUS_NEW,
		Price:    100,
		Quantity: 1,
	})

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_POST_ONLY,
		Quantity: 1,
		Price:    100,
	}
	if err := s.validateOrder(input); err != ErrPostOnlyWouldCross {
		t.Fatalf("expected ErrPostOnlyWouldCross, got %v", err)
	}
}

func TestValidateOrder_PostOnlyDoesNotCross(t *testing.T) {
	s, _ := newTestService()

	ob := orderbook.New()
	s.orderbooks[constants.CATEGORY_LINEAR]["BTCUSDT"] = ob
	ob.Add(&types.Order{
		ID:       1,
		UserID:   2,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Status:   constants.ORDER_STATUS_NEW,
		Price:    105,
		Quantity: 1,
	})

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_POST_ONLY,
		Quantity: 1,
		Price:    100,
	}
	if err := s.validateOrder(input); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidateOrder_PostOnlyEqualPriceRejected(t *testing.T) {
	s, _ := newTestService()

	ob := orderbook.New()
	s.orderbooks[constants.CATEGORY_LINEAR]["BTCUSDT"] = ob
	ob.Add(&types.Order{
		ID:       1,
		UserID:   2,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Status:   constants.ORDER_STATUS_NEW,
		Price:    100,
		Quantity: 1,
	})

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_POST_ONLY,
		Quantity: 1,
		Price:    100,
	}
	if err := s.validateOrder(input); err != ErrPostOnlyWouldCross {
		t.Fatalf("expected ErrPostOnlyWouldCross, got %v", err)
	}
}

func TestPlaceOrder_PostOnlyRejectsBeforeReserve(t *testing.T) {
	clearing := &countingClearing{}
	s, _ := newTestServiceWithClearing(clearing)

	ob := orderbook.New()
	s.orderbooks[constants.CATEGORY_LINEAR]["BTCUSDT"] = ob
	ob.Add(&types.Order{
		ID:       1,
		UserID:   2,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Status:   constants.ORDER_STATUS_NEW,
		Price:    100,
		Quantity: 1,
	})

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_POST_ONLY,
		Quantity: 1,
		Price:    100,
	}

	if _, err := s.PlaceOrder(context.Background(), input); err != ErrPostOnlyWouldCross {
		t.Fatalf("expected ErrPostOnlyWouldCross, got %v", err)
	}
	if clearing.reserveCalls != 0 {
		t.Fatalf("expected no reserve calls, got %d", clearing.reserveCalls)
	}
}

func TestValidateOCO_ChildTriggersRequired(t *testing.T) {
	s, portfolio := newTestService()
	portfolio.addPosition(1, "BTCUSDT", 1, constants.SIDE_LONG)

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 1,
		Price:    100,
		OCO: &types.OCOInput{
			TakeProfit: types.OCOChildOrder{TriggerPrice: 0, Price: 99, ReduceOnly: true},
			StopLoss:   types.OCOChildOrder{TriggerPrice: 90, Price: 91, ReduceOnly: true},
		},
	}
	if err := s.validateOCO(input); err != ErrInvalidTriggerPrice {
		t.Fatalf("expected ErrInvalidTriggerPrice, got %v", err)
	}
}

func TestValidateOrder_TriggerPriceRules(t *testing.T) {
	s, _ := newTestService()
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
		TriggerPrice: 100,
	}
	if err := s.validateOrder(buy); err != ErrInvalidTriggerPrice {
		t.Fatalf("expected ErrInvalidTriggerPrice for BUY trigger >= price, got %v", err)
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
		TriggerPrice: 100,
	}
	if err := s.validateOrder(sell); err != ErrInvalidTriggerPrice {
		t.Fatalf("expected ErrInvalidTriggerPrice for SELL trigger <= price, got %v", err)
	}
}

func TestValidateOrder_ReduceOnlyRequiresPosition(t *testing.T) {
	s, _ := newTestService()

	input := &types.OrderInput{
		UserID:     1,
		Symbol:     "BTCUSDT",
		Category:   constants.CATEGORY_LINEAR,
		Side:       constants.ORDER_SIDE_SELL,
		Type:       constants.ORDER_TYPE_LIMIT,
		TIF:        constants.TIF_GTC,
		Quantity:   1,
		Price:      100,
		ReduceOnly: true,
	}
	if err := s.validateOrder(input); err != ErrReduceOnlyNoPosition {
		t.Fatalf("expected ErrReduceOnlyNoPosition, got %v", err)
	}
}

func TestValidateOrder_ReduceOnlySideMismatch(t *testing.T) {
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
		t.Fatalf("expected ErrReduceOnlySide, got %v", err)
	}
}

func TestValidateOrder_ReduceOnlyCommitmentExceeded(t *testing.T) {
	s, portfolio := newTestService()
	portfolio.addPosition(1, "BTCUSDT", 1, constants.SIDE_LONG)
	s.reduceOnlyCommitment[1] = map[string]int64{"BTCUSDT": 1}

	input := &types.OrderInput{
		UserID:     1,
		Symbol:     "BTCUSDT",
		Category:   constants.CATEGORY_LINEAR,
		Side:       constants.ORDER_SIDE_SELL,
		Type:       constants.ORDER_TYPE_LIMIT,
		TIF:        constants.TIF_GTC,
		Quantity:   1,
		Price:      100,
		ReduceOnly: true,
	}
	if err := s.validateOrder(input); err != ErrReduceOnlyCommitmentExceeded {
		t.Fatalf("expected ErrReduceOnlyCommitmentExceeded, got %v", err)
	}
}

func TestReduceOnlyCommitmentUpdates(t *testing.T) {
	clearing := &countingClearing{}
	s, portfolio := newTestServiceWithClearing(clearing)
	portfolio.addPosition(1, "BTCUSDT", 2, constants.SIDE_LONG)

	input := &types.OrderInput{
		UserID:     1,
		Symbol:     "BTCUSDT",
		Category:   constants.CATEGORY_LINEAR,
		Side:       constants.ORDER_SIDE_SELL,
		Type:       constants.ORDER_TYPE_LIMIT,
		TIF:        constants.TIF_GTC,
		Quantity:   2,
		Price:      100,
		ReduceOnly: true,
	}

	result, err := s.PlaceOrder(context.Background(), input)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if s.reduceOnlyCommitment[1]["BTCUSDT"] != 2 {
		t.Fatalf("expected commitment 2, got %d", s.reduceOnlyCommitment[1]["BTCUSDT"])
	}

	s.cancelOrder(result.Orders[0])
	if s.reduceOnlyCommitment[1]["BTCUSDT"] != 0 {
		t.Fatalf("expected commitment 0 after cancel, got %d", s.reduceOnlyCommitment[1]["BTCUSDT"])
	}
}

func TestValidateOrder_OCOOnSpotRejected(t *testing.T) {
	s, _ := newTestService()
	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_SPOT,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 1,
		Price:    100,
		OCO: &types.OCOInput{
			TakeProfit: types.OCOChildOrder{TriggerPrice: 120, Price: 119, ReduceOnly: true},
			StopLoss:   types.OCOChildOrder{TriggerPrice: 80, Price: 81, ReduceOnly: true},
		},
	}
	if err := s.validateOCO(input); err != ErrOCOSpot {
		t.Fatalf("expected ErrOCOSpot, got %v", err)
	}
}

func TestCheckSelfMatchMarket(t *testing.T) {
	s, _ := newTestService()

	order := &types.Order{
		ID:       10,
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
	s.storeOrder(order)

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_MARKET,
		TIF:      constants.TIF_IOC,
		Quantity: 1,
	}

	if err := s.checkSelfMatch(input); err != ErrSelfMatch {
		t.Fatalf("expected ErrSelfMatch, got %v", err)
	}
}

func TestCheckSelfMatchLimit(t *testing.T) {
	s, _ := newTestService()

	ob := orderbook.New()
	s.orderbooks[constants.CATEGORY_LINEAR]["BTCUSDT"] = ob

	ask := &types.Order{
		ID:       10,
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
	ob.Add(ask)
	s.storeOrder(ask)

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

	if err := s.checkSelfMatch(input); err != ErrSelfMatch {
		t.Fatalf("expected ErrSelfMatch, got %v", err)
	}
}

func TestValidateOrder_QuantityZeroRules(t *testing.T) {
	s, _ := newTestService()

	regular := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 0,
		Price:    100,
	}
	if err := s.validateOrder(regular); err != ErrInvalidQuantity {
		t.Fatalf("expected ErrInvalidQuantity, got %v", err)
	}

	conditional := &types.OrderInput{
		UserID:       1,
		Symbol:       "BTCUSDT",
		Category:     constants.CATEGORY_LINEAR,
		Side:         constants.ORDER_SIDE_BUY,
		Type:         constants.ORDER_TYPE_LIMIT,
		TIF:          constants.TIF_GTC,
		Quantity:     0,
		Price:        100,
		TriggerPrice: 90,
	}
	if err := s.validateOrder(conditional); err != nil {
		t.Fatalf("expected no error for conditional qty=0, got %v", err)
	}
}

func TestValidateOCO_Rules(t *testing.T) {
	s, portfolio := newTestService()

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 1,
		Price:    100,
		OCO: &types.OCOInput{
			TakeProfit: types.OCOChildOrder{TriggerPrice: 120, Price: 119, ReduceOnly: true},
			StopLoss:   types.OCOChildOrder{TriggerPrice: 110, Price: 111, ReduceOnly: true},
		},
	}

	if err := s.validateOCO(input); err != ErrOCONoPosition {
		t.Fatalf("expected ErrOCONoPosition, got %v", err)
	}

	portfolio.addPosition(1, "BTCUSDT", 10, constants.SIDE_LONG)
	input.OCO.TakeProfit.TriggerPrice = 110
	input.OCO.StopLoss.TriggerPrice = 120
	if err := s.validateOCO(input); err != ErrOCOTPTriggerInvalid {
		t.Fatalf("expected ErrOCOTPTriggerInvalid, got %v", err)
	}

	input.OCO.TakeProfit.TriggerPrice = 130
	input.OCO.StopLoss.TriggerPrice = 120
	if err := s.validateOCO(input); err != nil {
		t.Fatalf("expected no error for LONG with TP>SL, got %v", err)
	}

	portfolio.addPosition(1, "BTCUSDT", 10, constants.SIDE_SHORT)
	input.OCO.TakeProfit.TriggerPrice = 120
	input.OCO.StopLoss.TriggerPrice = 130
	if err := s.validateOCO(input); err != nil {
		t.Fatalf("expected no error for SHORT with TP<SL, got %v", err)
	}

	input.OCO.TakeProfit.TriggerPrice = 140
	if err := s.validateOCO(input); err != ErrOCOSLTriggerInvalid {
		t.Fatalf("expected ErrOCOSLTriggerInvalid, got %v", err)
	}
}

func TestPlaceOrder_FOKPrecheckRejects(t *testing.T) {
	clearing := &countingClearing{}
	s, _ := newTestServiceWithClearing(clearing)

	ob := orderbook.New()
	s.orderbooks[constants.CATEGORY_LINEAR]["BTCUSDT"] = ob

	maker := &types.Order{
		ID:       1,
		UserID:   2,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Status:   constants.ORDER_STATUS_NEW,
		Price:    100,
		Quantity: 1,
	}
	ob.Add(maker)

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_FOK,
		Quantity: 2,
		Price:    100,
	}

	if _, err := s.PlaceOrder(context.Background(), input); err != ErrFOKInsufficientLiquidity {
		t.Fatalf("expected ErrFOKInsufficientLiquidity, got %v", err)
	}
	if clearing.reserveCalls != 0 {
		t.Fatalf("expected no reserve calls, got %d", clearing.reserveCalls)
	}
	if len(s.orders) != 0 {
		t.Fatalf("expected no stored orders on FOK rejection")
	}
}

func TestPlaceOrder_OCOCreation(t *testing.T) {
	s, portfolio := newTestService()
	portfolio.addPosition(1, "BTCUSDT", 10, constants.SIDE_LONG)

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 10,
		Price:    0,
		OCO: &types.OCOInput{
			Quantity: 0,
			TakeProfit: types.OCOChildOrder{
				TriggerPrice: 60000,
				Price:        59900,
				ReduceOnly:   true,
			},
			StopLoss: types.OCOChildOrder{
				TriggerPrice: 45000,
				Price:        45100,
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
	if tp.OrderLinkId == 0 || sl.OrderLinkId == 0 || tp.OrderLinkId != sl.OrderLinkId {
		t.Fatalf("expected shared OrderLinkId >0, got %d and %d", tp.OrderLinkId, sl.OrderLinkId)
	}
	if !tp.IsConditional || !sl.IsConditional {
		t.Fatalf("expected OCO orders to be conditional")
	}
	if !tp.CloseOnTrigger || !sl.CloseOnTrigger {
		t.Fatalf("expected CloseOnTrigger=true for OCO orders")
	}
	if !tp.ReduceOnly || !sl.ReduceOnly {
		t.Fatalf("expected ReduceOnly=true for OCO orders")
	}
	if tp.Quantity != 0 || sl.Quantity != 0 {
		t.Fatalf("expected quantity=0 for OCO orders, got %d and %d", tp.Quantity, sl.Quantity)
	}
	if tp.StopOrderType != constants.STOP_ORDER_TYPE_TAKE_PROFIT {
		t.Fatalf("expected TP stopOrderType, got %d", tp.StopOrderType)
	}
	if sl.StopOrderType != constants.STOP_ORDER_TYPE_STOP_LOSS {
		t.Fatalf("expected SL stopOrderType, got %d", sl.StopOrderType)
	}
}

func TestPlaceOrder_ConditionalSkipsReserve(t *testing.T) {
	clearing := &countingClearing{}
	s, _ := newTestServiceWithClearing(clearing)

	input := &types.OrderInput{
		UserID:       1,
		Symbol:       "BTCUSDT",
		Category:     constants.CATEGORY_LINEAR,
		Side:         constants.ORDER_SIDE_BUY,
		Type:         constants.ORDER_TYPE_LIMIT,
		TIF:          constants.TIF_GTC,
		Quantity:     1,
		Price:        100,
		TriggerPrice: 90,
	}

	if _, err := s.PlaceOrder(context.Background(), input); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if clearing.reserveCalls != 0 {
		t.Fatalf("expected no reserve calls, got %d", clearing.reserveCalls)
	}
}

func TestCreateChildOrderInput_CloseOnTriggerUsesPositionSize(t *testing.T) {
	s, portfolio := newTestService()
	portfolio.addPosition(1, "BTCUSDT", 5, constants.SIDE_LONG)

	triggered := &types.Order{
		UserID:         1,
		Symbol:         "BTCUSDT",
		Category:       constants.CATEGORY_LINEAR,
		Side:           constants.ORDER_SIDE_BUY,
		Type:           constants.ORDER_TYPE_LIMIT,
		TIF:            constants.TIF_GTC,
		Quantity:       0,
		Price:          48000,
		TriggerPrice:   49000,
		CloseOnTrigger: true,
	}

	child := s.createChildOrderInput(triggered)
	if child == nil {
		t.Fatalf("expected child order input")
	}
	if child.Quantity != 5 {
		t.Fatalf("expected quantity from position size, got %d", child.Quantity)
	}
	if !child.ReduceOnly {
		t.Fatalf("expected reduceOnly=true for closeOnTrigger child")
	}
	if child.Side != constants.ORDER_SIDE_SELL {
		t.Fatalf("expected opposite side, got %d", child.Side)
	}
	if child.Type != constants.ORDER_TYPE_LIMIT || child.Price != 48000 {
		t.Fatalf("expected limit child with same price, got type %d price %d", child.Type, child.Price)
	}
	if child.TIF != constants.TIF_GTC {
		t.Fatalf("expected GTC for limit child, got %d", child.TIF)
	}
}

func TestPlaceOrder_FOKFullyFills(t *testing.T) {
	clearing := &countingClearing{}
	s, _ := newTestServiceWithClearing(clearing)

	ob := orderbook.New()
	s.orderbooks[constants.CATEGORY_LINEAR]["BTCUSDT"] = ob

	for i := 0; i < 3; i++ {
		ob.Add(&types.Order{
			ID:       types.OrderID(i + 1),
			UserID:   2,
			Symbol:   "BTCUSDT",
			Category: constants.CATEGORY_LINEAR,
			Side:     constants.ORDER_SIDE_SELL,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Status:   constants.ORDER_STATUS_NEW,
			Price:    100,
			Quantity: 1,
		})
	}

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_FOK,
		Quantity: 3,
		Price:    100,
	}

	result, err := s.PlaceOrder(context.Background(), input)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Status != constants.ORDER_STATUS_FILLED {
		t.Fatalf("expected FILLED status, got %d", result.Status)
	}
	if result.Remaining != 0 {
		t.Fatalf("expected remaining 0, got %d", result.Remaining)
	}
	if clearing.reserveCalls != 1 {
		t.Fatalf("expected 1 reserve call, got %d", clearing.reserveCalls)
	}
}

func TestPlaceOrder_IOCPartialFill(t *testing.T) {
	clearing := &countingClearing{}
	s, _ := newTestServiceWithClearing(clearing)

	ob := orderbook.New()
	s.orderbooks[constants.CATEGORY_LINEAR]["BTCUSDT"] = ob

	ob.Add(&types.Order{
		ID:       1,
		UserID:   2,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Status:   constants.ORDER_STATUS_NEW,
		Price:    100,
		Quantity: 1,
	})

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_IOC,
		Quantity: 2,
		Price:    100,
	}

	result, err := s.PlaceOrder(context.Background(), input)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Status != constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED {
		t.Fatalf("expected PARTIALLY_FILLED_CANCELED, got %d", result.Status)
	}
	if result.Filled != 1 || result.Remaining != 1 {
		t.Fatalf("expected filled 1 remaining 1, got filled %d remaining %d", result.Filled, result.Remaining)
	}
	if clearing.reserveCalls != 1 {
		t.Fatalf("expected reserve called once, got %d", clearing.reserveCalls)
	}
}

func TestPlaceOrder_IOCFillsNothing(t *testing.T) {
	clearing := &countingClearing{}
	s, _ := newTestServiceWithClearing(clearing)

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_IOC,
		Quantity: 2,
		Price:    100,
	}

	result, err := s.PlaceOrder(context.Background(), input)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Status != constants.ORDER_STATUS_CANCELED {
		t.Fatalf("expected CANCELED, got %d", result.Status)
	}
	if result.Filled != 0 || result.Remaining != 2 {
		t.Fatalf("expected filled 0 remaining 2, got filled %d remaining %d", result.Filled, result.Remaining)
	}
	if clearing.reserveCalls != 1 {
		t.Fatalf("expected reserve called once, got %d", clearing.reserveCalls)
	}
}
func BenchmarkValidateOrder(b *testing.B) {
	b.ReportAllocs()
	s, _ := newTestService()
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
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = s.validateOrder(input)
	}
}
