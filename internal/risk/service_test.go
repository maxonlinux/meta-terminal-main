package risk

import (
	"context"
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

type MockOMS struct {
	orders []*types.OrderInput
}

func (m *MockOMS) PlaceOrder(ctx context.Context, input *types.OrderInput) (*types.OrderResult, error) {
	m.orders = append(m.orders, input)
	return &types.OrderResult{
		Orders: []*types.Order{{
			ID:       1,
			UserID:   input.UserID,
			Symbol:   input.Symbol,
			Category: input.Category,
			Side:     input.Side,
			Type:     input.Type,
			Status:   constants.ORDER_STATUS_NEW,
		}},
		Trades:    nil,
		Filled:    0,
		Remaining: input.Quantity,
		Status:    constants.ORDER_STATUS_NEW,
	}, nil
}

func TestCheckLiquidations_LongPosition(t *testing.T) {
	s := &Service{
		positions:  make(map[types.UserID]map[string]*types.Position),
		lastPrices: make(map[string]types.Price),
	}

	userID := types.UserID(1)
	s.positions[userID] = make(map[string]*types.Position)
	s.positions[userID]["BTCUSDT"] = &types.Position{
		Symbol:     "BTCUSDT",
		Size:       1,
		Side:       constants.ORDER_SIDE_BUY,
		EntryPrice: 50000,
		Leverage:   10,
	}

	var mockOMS MockOMS
	s.oms = &mockOMS

	s.checkLiquidations("BTCUSDT", 23000)

	if len(mockOMS.orders) != 1 {
		t.Errorf("Expected 1 liquidation order, got %d", len(mockOMS.orders))
	}

	if len(mockOMS.orders) > 0 {
		order := mockOMS.orders[0]
		if order.Symbol != "BTCUSDT" {
			t.Errorf("Expected symbol BTCUSDT, got %s", order.Symbol)
		}
		if order.Side != constants.ORDER_SIDE_SELL {
			t.Errorf("Expected side SELL, got %d", order.Side)
		}
		if order.ReduceOnly != true {
			t.Errorf("Expected reduceOnly=true")
		}
	}
}

func TestCheckLiquidations_ShortPosition(t *testing.T) {
	s := &Service{
		positions:  make(map[types.UserID]map[string]*types.Position),
		lastPrices: make(map[string]types.Price),
	}

	userID := types.UserID(1)
	s.positions[userID] = make(map[string]*types.Position)
	s.positions[userID]["BTCUSDT"] = &types.Position{
		Symbol:     "BTCUSDT",
		Size:       1,
		Side:       constants.ORDER_SIDE_SELL,
		EntryPrice: 50000,
		Leverage:   10,
	}

	var mockOMS MockOMS
	s.oms = &mockOMS

	s.checkLiquidations("BTCUSDT", 77000)

	if len(mockOMS.orders) != 1 {
		t.Errorf("Expected 1 liquidation order, got %d", len(mockOMS.orders))
	}

	if len(mockOMS.orders) > 0 {
		order := mockOMS.orders[0]
		if order.Symbol != "BTCUSDT" {
			t.Errorf("Expected symbol BTCUSDT, got %s", order.Symbol)
		}
		if order.Side != constants.ORDER_SIDE_BUY {
			t.Errorf("Expected side BUY, got %d", order.Side)
		}
	}
}

func TestCheckLiquidations_NoLiquidation(t *testing.T) {
	s := &Service{
		positions:  make(map[types.UserID]map[string]*types.Position),
		lastPrices: make(map[string]types.Price),
	}

	userID := types.UserID(1)
	s.positions[userID] = make(map[string]*types.Position)
	s.positions[userID]["BTCUSDT"] = &types.Position{
		Symbol:     "BTCUSDT",
		Size:       1,
		Side:       constants.ORDER_SIDE_BUY,
		EntryPrice: 50000,
		Leverage:   10,
	}

	var mockOMS MockOMS
	s.oms = &mockOMS

	s.checkLiquidations("BTCUSDT", 50000)

	if len(mockOMS.orders) != 0 {
		t.Errorf("Expected 0 liquidation orders, got %d", len(mockOMS.orders))
	}
}

func TestCalculateLiquidationPrice_Long(t *testing.T) {
	s := &Service{}

	pos := &types.Position{
		Symbol:     "BTCUSDT",
		Size:       1,
		Side:       constants.ORDER_SIDE_BUY,
		EntryPrice: 50000,
		Leverage:   10,
	}

	liqPrice := s.calculateLiquidationPrice(pos)

	expected := int64(50000) * int64(100-10*5) / 100
	if liqPrice != expected {
		t.Errorf("Expected liquidation price %d, got %d", expected, liqPrice)
	}
}

func TestCalculateLiquidationPrice_Short(t *testing.T) {
	s := &Service{}

	pos := &types.Position{
		Symbol:     "BTCUSDT",
		Size:       1,
		Side:       constants.ORDER_SIDE_SELL,
		EntryPrice: 50000,
		Leverage:   10,
	}

	liqPrice := s.calculateLiquidationPrice(pos)

	expected := int64(50000) + int64(50000)*int64(10*5)/100
	if liqPrice != expected {
		t.Errorf("Expected liquidation price %d, got %d", expected, liqPrice)
	}
}

func TestUpdatePosition(t *testing.T) {
	s := &Service{
		positions:  make(map[types.UserID]map[string]*types.Position),
		lastPrices: make(map[string]types.Price),
	}

	userID := types.UserID(1)
	pos := &types.Position{
		Symbol:     "BTCUSDT",
		Size:       1,
		Side:       constants.ORDER_SIDE_BUY,
		EntryPrice: 50000,
		Leverage:   10,
	}

	s.UpdatePosition(userID, pos)

	if s.positions[userID]["BTCUSDT"] == nil {
		t.Error("Expected position to be stored")
	}
}

func TestRemovePosition(t *testing.T) {
	s := &Service{
		positions:  make(map[types.UserID]map[string]*types.Position),
		lastPrices: make(map[string]types.Price),
	}

	userID := types.UserID(1)
	s.positions[userID] = make(map[string]*types.Position)
	s.positions[userID]["BTCUSDT"] = &types.Position{
		Symbol:     "BTCUSDT",
		Size:       1,
		Side:       constants.ORDER_SIDE_BUY,
		EntryPrice: 50000,
		Leverage:   10,
	}

	s.RemovePosition(userID, "BTCUSDT")

	if s.positions[userID]["BTCUSDT"] != nil {
		t.Error("Expected position to be removed")
	}
}

func BenchmarkCheckLiquidations(b *testing.B) {
	s := &Service{
		positions:  make(map[types.UserID]map[string]*types.Position),
		lastPrices: make(map[string]types.Price),
	}

	userID := types.UserID(1)
	s.positions[userID] = make(map[string]*types.Position)

	for i := 0; i < 1000; i++ {
		s.positions[userID][string(rune(i%256))] = &types.Position{
			Symbol:     string(rune(i % 256)),
			Size:       1,
			Side:       constants.ORDER_SIDE_BUY,
			EntryPrice: 50000,
			Leverage:   10,
		}
	}

	var mockOMS MockOMS
	s.oms = &mockOMS

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.checkLiquidations("a", 23000)
	}
}

func BenchmarkHandlePositionUpdate(b *testing.B) {
	s := &Service{
		positions:  make(map[types.UserID]map[string]*types.Position),
		lastPrices: make(map[string]types.Price),
	}

	data := []byte{
		1, 66, 84, 67, 85, 83, 68, 84, 0, 0, 0, 0, 0, 0, 0, 1,
		0, 0, 0, 0, 0, 0, 194, 133, 10, 0, 0, 0, 0, 0, 0, 0,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.handlePositionUpdate(data)
	}
}
