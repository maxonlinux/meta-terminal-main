package oms

import (
	"context"
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/orderbook"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

type testPortfolio struct {
	positions map[types.UserID]map[string]*types.Position
}

func (m *testPortfolio) GetPositions(userID types.UserID) []*types.Position {
	result := make([]*types.Position, 0)
	if positions, ok := m.positions[userID]; ok {
		for _, p := range positions {
			result = append(result, p)
		}
	}
	return result
}

func (m *testPortfolio) GetPosition(userID types.UserID, symbol string) *types.Position {
	if positions, ok := m.positions[userID]; ok {
		return positions[symbol]
	}
	return nil
}

func (m *testPortfolio) GetBalance(userID types.UserID, asset string) *types.UserBalance {
	return &types.UserBalance{
		Asset:     asset,
		Available: 1000000,
		Locked:    0,
		Margin:    0,
	}
}

func (m *testPortfolio) addPosition(userID types.UserID, symbol string, size int64, side int8) {
	if m.positions == nil {
		m.positions = make(map[types.UserID]map[string]*types.Position)
	}
	if m.positions[userID] == nil {
		m.positions[userID] = make(map[string]*types.Position)
	}
	m.positions[userID][symbol] = &types.Position{
		Symbol:     symbol,
		Size:       size,
		Side:       side,
		EntryPrice: 50000,
		Leverage:   10,
	}
}

type testClearing struct{}

func (m *testClearing) Reserve(userID types.UserID, symbol string, category int8, side int8, qty types.Quantity, price types.Price) error {
	return nil
}

func (m *testClearing) Release(userID types.UserID, symbol string, category int8, side int8, qty types.Quantity, price types.Price) {
}

func (m *testClearing) ExecuteTrade(trade *types.Trade, taker *types.Order, maker *types.Order) {
}

func newTestService() (*Service, *testPortfolio) {
	portfolio := &testPortfolio{
		positions: make(map[types.UserID]map[string]*types.Position),
	}
	clearing := &testClearing{}

	s, _ := New(Config{}, portfolio, clearing)
	return s, portfolio
}

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
		s.PlaceOrder(context.Background(), input)
	}
}

func BenchmarkSelfMatchCheck(b *testing.B) {
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
		s.checkSelfMatch(input)
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
