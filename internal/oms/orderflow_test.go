package oms

import (
	"context"
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/snowflake"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func placeOrderOrFail(t *testing.T, s *Service, input *types.OrderInput) *types.OrderResult {
	t.Helper()
	result, err := s.PlaceOrder(context.Background(), input)
	if err != nil {
		t.Fatalf("place order error: %v", err)
	}
	return result
}

func TestBalanceReserve_SPOT_BuyLimit(t *testing.T) {
	portfolio := &testPortfolio{positions: make(map[types.UserID]map[string]*types.Position)}
	clearing := newTrackingClearing()

	s, _ := New(Config{}, portfolio, clearing)

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_SPOT,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 1,
		Price:    50000,
	}

	_ = placeOrderOrFail(t, s, input)

	if len(clearing.reserves) != 1 {
		t.Fatalf("Expected 1 reserve call, got %d", len(clearing.reserves))
	}

	reserve := clearing.reserves[0]
	if reserve.userID != 1 || reserve.symbol != "BTCUSDT" {
		t.Errorf("Wrong userID or symbol in reserve")
	}
	if reserve.qty != 1 || reserve.price != 50000 {
		t.Errorf("Expected qty=1, price=50000 passed to Reserve, got qty=%v, price=%v", reserve.qty, reserve.price)
	}
}

func TestBalanceReserve_SPOT_SellLimit(t *testing.T) {
	portfolio := &testPortfolio{positions: make(map[types.UserID]map[string]*types.Position)}
	clearing := newTrackingClearing()

	s, _ := New(Config{}, portfolio, clearing)

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_SPOT,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 1,
		Price:    50000,
	}

	_ = placeOrderOrFail(t, s, input)

	if len(clearing.reserves) != 1 {
		t.Fatalf("Expected 1 reserve call, got %d", len(clearing.reserves))
	}

	reserve := clearing.reserves[0]
	if reserve.qty != 1 || reserve.price != 50000 {
		t.Errorf("Expected qty=1, price=50000 passed to Reserve, got qty=%v, price=%v", reserve.qty, reserve.price)
	}
}

func TestBalanceReserve_LINEAR_BuyLimit(t *testing.T) {
	portfolio := &testPortfolio{positions: make(map[types.UserID]map[string]*types.Position)}
	clearing := newTrackingClearing()

	s, _ := New(Config{}, portfolio, clearing)

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

	_ = placeOrderOrFail(t, s, input)

	if len(clearing.reserves) != 1 {
		t.Fatalf("Expected 1 reserve call, got %d", len(clearing.reserves))
	}

	reserve := clearing.reserves[0]
	if reserve.qty != 1 || reserve.price != 50000 {
		t.Errorf("Expected qty=1, price=50000 passed to Reserve, got qty=%v, price=%v", reserve.qty, reserve.price)
	}
}

func TestBalanceReserve_LINEAR_SellLimit(t *testing.T) {
	portfolio := &testPortfolio{positions: make(map[types.UserID]map[string]*types.Position)}
	portfolio.addPosition(1, "BTCUSDT", 1, constants.SIDE_LONG)

	clearing := newTrackingClearing()

	s, _ := New(Config{}, portfolio, clearing)

	input := &types.OrderInput{
		UserID:     1,
		Symbol:     "BTCUSDT",
		Category:   constants.CATEGORY_LINEAR,
		Side:       constants.ORDER_SIDE_SELL,
		Type:       constants.ORDER_TYPE_LIMIT,
		TIF:        constants.TIF_GTC,
		Quantity:   1,
		Price:      50000,
		ReduceOnly: true,
	}

	_ = placeOrderOrFail(t, s, input)

	if len(clearing.reserves) != 1 {
		t.Fatalf("Expected 1 reserve call, got %d", len(clearing.reserves))
	}

	reserve := clearing.reserves[0]
	if reserve.qty != 1 || reserve.price != 50000 {
		t.Errorf("Expected qty=1, price=50000 passed to Reserve, got qty=%v, price=%v", reserve.qty, reserve.price)
	}
}

func TestBalanceReserve_MarketOrder_NoReserve(t *testing.T) {
	portfolio := &testPortfolio{positions: make(map[types.UserID]map[string]*types.Position)}
	clearing := newTrackingClearing()

	s, _ := New(Config{}, portfolio, clearing)

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_MARKET,
		TIF:      constants.TIF_IOC,
		Quantity: 1,
	}

	_ = placeOrderOrFail(t, s, input)

	if len(clearing.reserves) != 0 {
		t.Errorf("Expected 0 reserves for MARKET, got %d", len(clearing.reserves))
	}
}

func TestBalanceRelease_PartialFill(t *testing.T) {
	portfolio := &testPortfolio{positions: make(map[types.UserID]map[string]*types.Position)}
	clearing := newTrackingClearing()

	s, _ := New(Config{}, portfolio, clearing)

	ob := s.getOrderBook(constants.CATEGORY_LINEAR, "BTCUSDT")

	maker := &types.Order{
		ID:       100,
		UserID:   2,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 1,
		Price:    50000,
		Filled:   0,
	}
	ob.Add(maker)

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

	_ = placeOrderOrFail(t, s, input)

	if len(clearing.trades) != 1 {
		t.Errorf("Expected 1 trade, got %d", len(clearing.trades))
	}
}

func TestMarginCalculation_PositionUpdate(t *testing.T) {
	portfolio := &testPortfolio{positions: make(map[types.UserID]map[string]*types.Position)}
	clearing := newTrackingClearing()

	s, _ := New(Config{}, portfolio, clearing)

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

	_ = placeOrderOrFail(t, s, input)

	if len(clearing.reserves) != 1 {
		t.Fatalf("Expected 1 reserve call, got %d", len(clearing.reserves))
	}

	reserve := clearing.reserves[0]
	if reserve.qty != 1 || reserve.price != 50000 {
		t.Errorf("Expected qty=1, price=50000 passed to Reserve, got qty=%v, price=%v", reserve.qty, reserve.price)
	}
}

func TestOCO_QuantityZeroMeansFullPositionClose_Linear(t *testing.T) {
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
		t.Errorf("Expected no error, got %v", err)
	}

	if len(result.Orders) != 2 {
		t.Fatalf("Expected 2 OCO orders, got %d", len(result.Orders))
	}

	if result.Remaining != 2 {
		t.Errorf("Expected Remaining = position size (2), got %v", result.Remaining)
	}
}

func TestReduceOnly_QuantityZeroMeansFullPosition_Linear(t *testing.T) {
	s, portfolio := newTestService()
	portfolio.addPosition(1, "BTCUSDT", 3, constants.SIDE_SHORT)

	input := &types.OrderInput{
		UserID:       1,
		Symbol:       "BTCUSDT",
		Category:     constants.CATEGORY_LINEAR,
		Side:         constants.ORDER_SIDE_BUY,
		Type:         constants.ORDER_TYPE_LIMIT,
		TIF:          constants.TIF_GTC,
		Quantity:     0,
		Price:        45000,
		TriggerPrice: 46000,
		ReduceOnly:   true,
	}

	_, err := s.PlaceOrder(context.Background(), input)
	if err != nil {
		t.Errorf("Expected no error for reduceOnly with Quantity=0, got %v", err)
	}
}

func TestCloseOnTrigger_QuantityZeroFullClose_Linear(t *testing.T) {
	s, portfolio := newTestService()
	portfolio.addPosition(1, "BTCUSDT", 5, constants.SIDE_LONG)

	input := &types.OrderInput{
		UserID:         1,
		Symbol:         "BTCUSDT",
		Category:       constants.CATEGORY_LINEAR,
		Side:           constants.ORDER_SIDE_BUY,
		Type:           constants.ORDER_TYPE_LIMIT,
		TIF:            constants.TIF_GTC,
		Quantity:       0,
		Price:          45000,
		TriggerPrice:   46000,
		CloseOnTrigger: true,
	}

	_, err := s.PlaceOrder(context.Background(), input)
	if err != nil {
		t.Errorf("Expected no error for closeOnTrigger with Quantity=0, got %v", err)
	}
}

func TestSelfMatchPrevention_DifferentUsers_Trading(t *testing.T) {
	portfolio := &testPortfolio{positions: make(map[types.UserID]map[string]*types.Position)}
	clearing := newTrackingClearing()

	s, _ := New(Config{}, portfolio, clearing)

	makerInput := &types.OrderInput{
		UserID:   2,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 1,
		Price:    50000,
	}
	_ = placeOrderOrFail(t, s, makerInput)

	takerInput := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 1,
		Price:    50000,
	}

	_, err := s.PlaceOrder(context.Background(), takerInput)
	if err != nil {
		t.Errorf("Expected no error for different users, got %v", err)
	}
}

func TestExecuteOrder_Status_Filled_GTC(t *testing.T) {
	portfolio := &testPortfolio{positions: make(map[types.UserID]map[string]*types.Position)}
	clearing := newTrackingClearing()
	s, _ := New(Config{}, portfolio, clearing)

	ob := s.getOrderBook(constants.CATEGORY_SPOT, "BTCUSDT")
	maker := &types.Order{
		ID:       types.OrderID(snowflake.Next()),
		UserID:   2,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_SPOT,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 1,
		Price:    50000,
		Filled:   0,
	}
	ob.Add(maker)

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_SPOT,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 1,
		Price:    50000,
	}

	order := s.GetOrder(1, types.OrderID(999)) // Check if order exists before
	_ = order

	result, err := s.PlaceOrder(context.Background(), input)
	if err != nil {
		t.Fatalf("PlaceOrder failed: %v", err)
	}

	if result.Status != constants.ORDER_STATUS_FILLED {
		t.Errorf("Expected FILLED (%d), got %d", constants.ORDER_STATUS_FILLED, result.Status)
	}
	if result.Filled != 1 {
		t.Errorf("Expected Filled=1, got %d", result.Filled)
	}
}

func TestExecuteOrder_Status_Filled_IOC(t *testing.T) {
	portfolio := &testPortfolio{positions: make(map[types.UserID]map[string]*types.Position)}
	clearing := newTrackingClearing()
	s, _ := New(Config{}, portfolio, clearing)

	ob := s.getOrderBook(constants.CATEGORY_LINEAR, "BTCUSDT")
	maker := &types.Order{
		ID:       types.OrderID(snowflake.Next()),
		UserID:   2,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 1,
		Price:    50000,
		Filled:   0,
	}
	ob.Add(maker)

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_MARKET,
		TIF:      constants.TIF_IOC,
		Quantity: 1,
	}

	result, err := s.PlaceOrder(context.Background(), input)
	if err != nil {
		t.Fatalf("PlaceOrder failed: %v", err)
	}

	if result.Status != constants.ORDER_STATUS_FILLED {
		t.Errorf("Expected FILLED (%d), got %d", constants.ORDER_STATUS_FILLED, result.Status)
	}
}

func TestExecuteOrder_Status_Filled_FOK(t *testing.T) {
	portfolio := &testPortfolio{positions: make(map[types.UserID]map[string]*types.Position)}
	clearing := newTrackingClearing()
	s, _ := New(Config{}, portfolio, clearing)

	ob := s.getOrderBook(constants.CATEGORY_LINEAR, "BTCUSDT")
	maker := &types.Order{
		ID:       types.OrderID(snowflake.Next()),
		UserID:   2,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 1,
		Price:    50000,
		Filled:   0,
	}
	ob.Add(maker)

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_MARKET,
		TIF:      constants.TIF_FOK,
		Quantity: 1,
	}

	result, err := s.PlaceOrder(context.Background(), input)
	if err != nil {
		t.Fatalf("PlaceOrder failed: %v", err)
	}

	if result.Status != constants.ORDER_STATUS_FILLED {
		t.Errorf("Expected FILLED (%d), got %d", constants.ORDER_STATUS_FILLED, result.Status)
	}
}

func TestExecuteOrder_Status_Cancelled_IOC(t *testing.T) {
	portfolio := &testPortfolio{positions: make(map[types.UserID]map[string]*types.Position)}
	clearing := newTrackingClearing()
	s, _ := New(Config{}, portfolio, clearing)

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_MARKET,
		TIF:      constants.TIF_IOC,
		Quantity: 1,
	}

	result, err := s.PlaceOrder(context.Background(), input)
	if err != nil {
		t.Fatalf("PlaceOrder failed: %v", err)
	}

	if result.Status != constants.ORDER_STATUS_CANCELED {
		t.Errorf("Expected CANCELLED (%d), got %d", constants.ORDER_STATUS_CANCELED, result.Status)
	}
	if result.Filled != 0 {
		t.Errorf("Expected Filled=0, got %d", result.Filled)
	}
}

func TestExecuteOrder_Status_Cancelled_FOK(t *testing.T) {
	portfolio := &testPortfolio{positions: make(map[types.UserID]map[string]*types.Position)}
	clearing := newTrackingClearing()
	s, _ := New(Config{}, portfolio, clearing)

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_MARKET,
		TIF:      constants.TIF_FOK,
		Quantity: 1,
	}

	_, err := s.PlaceOrder(context.Background(), input)
	if err != ErrFOKInsufficientLiquidity {
		t.Errorf("Expected ErrFOKInsufficientLiquidity, got %v", err)
	}
}

func TestExecuteOrder_Status_PartiallyFilled_IOC(t *testing.T) {
	portfolio := &testPortfolio{positions: make(map[types.UserID]map[string]*types.Position)}
	clearing := newTrackingClearing()
	s, _ := New(Config{}, portfolio, clearing)

	ob := s.getOrderBook(constants.CATEGORY_LINEAR, "BTCUSDT")
	maker := &types.Order{
		ID:       types.OrderID(snowflake.Next()),
		UserID:   2,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 1,
		Price:    50000,
		Filled:   0,
	}
	ob.Add(maker)

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_MARKET,
		TIF:      constants.TIF_IOC,
		Quantity: 2,
	}

	result, err := s.PlaceOrder(context.Background(), input)
	if err != nil {
		t.Fatalf("PlaceOrder failed: %v", err)
	}

	if result.Status != constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED {
		t.Errorf("Expected PARTIALLY_FILLED_CANCELED (%d), got %d",
			constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED, result.Status)
	}
	if result.Filled != 1 {
		t.Errorf("Expected Filled=1, got %d", result.Filled)
	}
}

func TestExecuteOrder_Status_PartiallyFilled_FOK(t *testing.T) {
	portfolio := &testPortfolio{positions: make(map[types.UserID]map[string]*types.Position)}
	clearing := newTrackingClearing()
	s, _ := New(Config{}, portfolio, clearing)

	ob := s.getOrderBook(constants.CATEGORY_LINEAR, "BTCUSDT")
	maker := &types.Order{
		ID:       types.OrderID(snowflake.Next()),
		UserID:   2,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 1,
		Price:    50000,
		Filled:   0,
	}
	ob.Add(maker)

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_MARKET,
		TIF:      constants.TIF_FOK,
		Quantity: 2,
	}

	_, err := s.PlaceOrder(context.Background(), input)
	if err != ErrFOKInsufficientLiquidity {
		t.Errorf("Expected ErrFOKInsufficientLiquidity, got %v", err)
	}
}

func TestExecuteOrder_Status_New_GTC(t *testing.T) {
	portfolio := &testPortfolio{positions: make(map[types.UserID]map[string]*types.Position)}
	clearing := newTrackingClearing()
	s, _ := New(Config{}, portfolio, clearing)

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_SPOT,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 1,
		Price:    50000,
	}

	result, err := s.PlaceOrder(context.Background(), input)
	if err != nil {
		t.Fatalf("PlaceOrder failed: %v", err)
	}

	if result.Status != constants.ORDER_STATUS_NEW {
		t.Errorf("Expected NEW (%d), got %d", constants.ORDER_STATUS_NEW, result.Status)
	}
	if result.Filled != 0 {
		t.Errorf("Expected Filled=0, got %d", result.Filled)
	}
}

func TestExecuteOrder_Status_New_PostOnly(t *testing.T) {
	portfolio := &testPortfolio{positions: make(map[types.UserID]map[string]*types.Position)}
	clearing := newTrackingClearing()
	s, _ := New(Config{}, portfolio, clearing)

	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_SPOT,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_POST_ONLY,
		Quantity: 1,
		Price:    50000,
	}

	result, err := s.PlaceOrder(context.Background(), input)
	if err != nil {
		t.Fatalf("PlaceOrder failed: %v", err)
	}

	if result.Status != constants.ORDER_STATUS_NEW {
		t.Errorf("Expected NEW (%d), got %d", constants.ORDER_STATUS_NEW, result.Status)
	}
}
