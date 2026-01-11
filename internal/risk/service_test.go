package risk

import (
	"context"
	"io"
	"log"
	"os"
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func TestMain(m *testing.M) {
	prev := log.Writer()
	log.SetOutput(io.Discard)
	code := m.Run()
	log.SetOutput(prev)
	os.Exit(code)
}

type MockOMS struct {
	orders []types.OrderInput
}

func (m *MockOMS) PlaceOrder(ctx context.Context, input *types.OrderInput) (*types.OrderResult, error) {
	m.orders = append(m.orders, *input)
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
		positionsByUser:   make(map[types.UserID]map[string]*types.Position),
		positionsBySymbol: make(map[string]map[types.UserID]*types.Position),
		lastPrices:        make(map[string]types.Price),
	}

	userID := types.UserID(1)
	s.UpdatePosition(userID, &types.Position{
		Symbol:     "BTCUSDT",
		Size:       1,
		Side:       constants.ORDER_SIDE_BUY,
		EntryPrice: 50000,
		Leverage:   10,
	})

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
		positionsByUser:   make(map[types.UserID]map[string]*types.Position),
		positionsBySymbol: make(map[string]map[types.UserID]*types.Position),
		lastPrices:        make(map[string]types.Price),
	}

	userID := types.UserID(1)
	s.UpdatePosition(userID, &types.Position{
		Symbol:     "BTCUSDT",
		Size:       1,
		Side:       constants.ORDER_SIDE_SELL,
		EntryPrice: 50000,
		Leverage:   10,
	})

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
		positionsByUser:   make(map[types.UserID]map[string]*types.Position),
		positionsBySymbol: make(map[string]map[types.UserID]*types.Position),
		lastPrices:        make(map[string]types.Price),
	}

	userID := types.UserID(1)
	s.UpdatePosition(userID, &types.Position{
		Symbol:     "BTCUSDT",
		Size:       1,
		Side:       constants.ORDER_SIDE_BUY,
		EntryPrice: 50000,
		Leverage:   10,
	})

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
		positionsByUser:   make(map[types.UserID]map[string]*types.Position),
		positionsBySymbol: make(map[string]map[types.UserID]*types.Position),
		lastPrices:        make(map[string]types.Price),
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

	if s.positionsByUser[userID]["BTCUSDT"] == nil {
		t.Error("Expected position to be stored")
	}
}

func TestRemovePosition(t *testing.T) {
	s := &Service{
		positionsByUser:   make(map[types.UserID]map[string]*types.Position),
		positionsBySymbol: make(map[string]map[types.UserID]*types.Position),
		lastPrices:        make(map[string]types.Price),
	}

	userID := types.UserID(1)
	s.UpdatePosition(userID, &types.Position{
		Symbol:     "BTCUSDT",
		Size:       1,
		Side:       constants.ORDER_SIDE_BUY,
		EntryPrice: 50000,
		Leverage:   10,
	})

	s.RemovePosition(userID, "BTCUSDT")

	if s.positionsByUser[userID]["BTCUSDT"] != nil {
		t.Error("Expected position to be removed")
	}
}

type nopOMS struct {
	result *types.OrderResult
	order  types.Order
}

func (n *nopOMS) PlaceOrder(ctx context.Context, input *types.OrderInput) (*types.OrderResult, error) {
	n.order.ID = 1
	n.order.UserID = input.UserID
	n.order.Symbol = input.Symbol
	n.order.Category = input.Category
	n.order.Side = input.Side
	n.order.Type = input.Type
	n.order.Status = constants.ORDER_STATUS_NEW
	n.result.Orders[0] = &n.order
	n.result.Filled = 0
	n.result.Remaining = input.Quantity
	n.result.Status = constants.ORDER_STATUS_NEW
	return n.result, nil
}

func BenchmarkCheckLiquidations_NoMatch(b *testing.B) {
	b.ReportAllocs()
	prev := log.Writer()
	log.SetOutput(io.Discard)
	defer log.SetOutput(prev)
	s := &Service{
		positionsByUser:   make(map[types.UserID]map[string]*types.Position),
		positionsBySymbol: make(map[string]map[types.UserID]*types.Position),
		lastPrices:        make(map[string]types.Price),
	}

	userID := types.UserID(1)
	for i := 0; i < 1000; i++ {
		s.UpdatePosition(userID, &types.Position{
			Symbol:     string(rune(i % 256)),
			Size:       1,
			Side:       constants.ORDER_SIDE_BUY,
			EntryPrice: 50000,
			Leverage:   10,
		})
	}

	emptyResult := &types.OrderResult{Orders: make([]*types.Order, 1)}
	s.oms = &nopOMS{result: emptyResult}

	s.checkLiquidations("z", 50000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.checkLiquidations("z", 50000)
	}
}

func BenchmarkCheckLiquidations_Match(b *testing.B) {
	b.ReportAllocs()
	prev := log.Writer()
	log.SetOutput(io.Discard)
	defer log.SetOutput(prev)
	s := &Service{
		positionsByUser:   make(map[types.UserID]map[string]*types.Position),
		positionsBySymbol: make(map[string]map[types.UserID]*types.Position),
		lastPrices:        make(map[string]types.Price),
	}

	for i := 0; i < 1000; i++ {
		userID := types.UserID(i + 1)
		s.UpdatePosition(userID, &types.Position{
			Symbol:     "BTCUSDT",
			Size:       1,
			Side:       constants.ORDER_SIDE_BUY,
			EntryPrice: 50000,
			Leverage:   10,
		})
	}

	emptyResult := &types.OrderResult{Orders: make([]*types.Order, 1)}
	s.oms = &nopOMS{result: emptyResult}

	s.checkLiquidations("BTCUSDT", 23000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.checkLiquidations("BTCUSDT", 23000)
	}
}

func BenchmarkOnPositionUpdate(b *testing.B) {
	b.ReportAllocs()
	prev := log.Writer()
	log.SetOutput(io.Discard)
	defer log.SetOutput(prev)
	s := &Service{
		positionsByUser:   make(map[types.UserID]map[string]*types.Position),
		positionsBySymbol: make(map[string]map[types.UserID]*types.Position),
		lastPrices:        make(map[string]types.Price),
	}

	s.OnPositionUpdate(1, "BTCUSDT", 1, constants.ORDER_SIDE_BUY, 50000, 10)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.OnPositionUpdate(1, "BTCUSDT", 1, constants.ORDER_SIDE_BUY, 50000, 10)
	}
}
