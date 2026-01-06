package pricefeed

import (
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/position"
	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

type mockEngine struct {
	closePositionCalled bool
	lastUserID          types.UserID
	lastSymbol          types.SymbolID
}

func (m *mockEngine) ClosePosition(userID types.UserID, symbol types.SymbolID) error {
	m.closePositionCalled = true
	m.lastUserID = userID
	m.lastSymbol = symbol
	return nil
}

func TestShouldLiquidate_ZeroSize(t *testing.T) {
	pf := &PriceFeed{}

	pos := &types.Position{
		Size: 0,
	}

	result := pf.shouldLiquidate(pos, 50000)
	if result {
		t.Error("Expected shouldLiquidate to return false for zero size position")
	}
}

func TestShouldLiquidate_ZeroMargin(t *testing.T) {
	pf := &PriceFeed{}

	pos := &types.Position{
		Size:              10,
		Side:              constants.ORDER_SIDE_BUY,
		EntryPrice:        50000,
		InitialMargin:     0,
		MaintenanceMargin: 0,
	}

	result := pf.shouldLiquidate(pos, 50000)
	if result {
		t.Error("Expected shouldLiquidate to return false for zero margin position")
	}
}

func TestShouldLiquidate_LongProfit(t *testing.T) {
	pf := &PriceFeed{}

	pos := &types.Position{
		Size:              10,
		Side:              constants.ORDER_SIDE_BUY,
		EntryPrice:        50000,
		InitialMargin:     5000000,
		MaintenanceMargin: 500000,
	}

	result := pf.shouldLiquidate(pos, 55000)
	if result {
		t.Error("Expected shouldLiquidate to return false for profitable position")
	}
}

func TestShouldLiquidate_LongLoss_BeforeLiquidation(t *testing.T) {
	pf := &PriceFeed{}

	pos := &types.Position{
		Size:              10,
		Side:              constants.ORDER_SIDE_BUY,
		EntryPrice:        50000,
		InitialMargin:     5000000,
		MaintenanceMargin: 500000,
	}

	buffer := pos.InitialMargin - pos.MaintenanceMargin
	lossPrice := types.Price(int64(pos.EntryPrice) - int64(buffer)/int64(pos.Size) + 1)

	result := pf.shouldLiquidate(pos, lossPrice)
	if result {
		t.Error("Expected shouldLiquidate to return false before liquidation price")
	}
}

func TestShouldLiquidate_LongAtLiquidation(t *testing.T) {
	pf := &PriceFeed{}

	pos := &types.Position{
		Size:              10,
		Side:              constants.ORDER_SIDE_BUY,
		EntryPrice:        50000,
		InitialMargin:     5000000,
		MaintenanceMargin: 500000,
	}

	buffer := pos.InitialMargin - pos.MaintenanceMargin
	liqPrice := types.Price(int64(pos.EntryPrice) - int64(buffer)/int64(pos.Size))

	result := pf.shouldLiquidate(pos, liqPrice)
	if result {
		t.Error("Expected shouldLiquidate to return false at exact liquidation price (buffer boundary)")
	}
}

func TestShouldLiquidate_LongAfterLiquidation(t *testing.T) {
	pf := &PriceFeed{}

	pos := &types.Position{
		Size:              10,
		Side:              constants.ORDER_SIDE_BUY,
		EntryPrice:        50000,
		InitialMargin:     5000000,
		MaintenanceMargin: 500000,
	}

	buffer := pos.InitialMargin - pos.MaintenanceMargin
	liqPrice := types.Price(int64(pos.EntryPrice) - int64(buffer)/int64(pos.Size) - 1)

	result := pf.shouldLiquidate(pos, liqPrice)
	if !result {
		t.Error("Expected shouldLiquidate to return true after liquidation price")
	}
}

func TestShouldLiquidate_ShortProfit(t *testing.T) {
	pf := &PriceFeed{}

	pos := &types.Position{
		Size:              10,
		Side:              constants.ORDER_SIDE_SELL,
		EntryPrice:        50000,
		InitialMargin:     5000000,
		MaintenanceMargin: 500000,
	}

	result := pf.shouldLiquidate(pos, 45000)
	if result {
		t.Error("Expected shouldLiquidate to return false for profitable short position")
	}
}

func TestShouldLiquidate_ShortLoss_BeforeLiquidation(t *testing.T) {
	pf := &PriceFeed{}

	pos := &types.Position{
		Size:              10,
		Side:              constants.ORDER_SIDE_SELL,
		EntryPrice:        50000,
		InitialMargin:     5000000,
		MaintenanceMargin: 500000,
	}

	buffer := pos.InitialMargin - pos.MaintenanceMargin
	lossPrice := types.Price(int64(pos.EntryPrice) + int64(buffer)/int64(pos.Size) - 1)

	result := pf.shouldLiquidate(pos, lossPrice)
	if result {
		t.Error("Expected shouldLiquidate to return false before liquidation price for short")
	}
}

func TestShouldLiquidate_ShortAfterLiquidation(t *testing.T) {
	pf := &PriceFeed{}

	pos := &types.Position{
		Size:              10,
		Side:              constants.ORDER_SIDE_SELL,
		EntryPrice:        50000,
		InitialMargin:     5000000,
		MaintenanceMargin: 500000,
	}

	buffer := pos.InitialMargin - pos.MaintenanceMargin
	liqPrice := types.Price(int64(pos.EntryPrice) + int64(buffer)/int64(pos.Size) + 1)

	result := pf.shouldLiquidate(pos, liqPrice)
	if !result {
		t.Error("Expected shouldLiquidate to return true after liquidation price for short")
	}
}

func TestUpdatePrice_NoPositions(t *testing.T) {
	s := &state.State{
		Users:   make(map[types.UserID]*state.UserState),
		Symbols: make(map[types.SymbolID]*state.OrderBookState),
	}
	eng := &mockEngine{}
	pf := NewPriceFeed(s, eng)

	pf.UpdatePrice(1, 50000)

	if eng.closePositionCalled {
		t.Error("Expected ClosePosition to not be called when there are no positions")
	}
}

func TestUpdatePrice_NoLiquidationNeeded(t *testing.T) {
	s := &state.State{
		Users: map[types.UserID]*state.UserState{
			1: {
				Balances:  make(map[string]*types.UserBalance),
				Positions: make(map[types.SymbolID]*types.Position),
			},
		},
		Symbols: make(map[types.SymbolID]*state.OrderBookState),
	}

	pos := &types.Position{
		UserID:            1,
		Symbol:            1,
		Size:              10,
		Side:              constants.ORDER_SIDE_BUY,
		EntryPrice:        50000,
		Leverage:          10,
		InitialMargin:     5000000,
		MaintenanceMargin: 500000,
	}
	position.CalculatePositionRisk(pos)

	s.Users[1].Positions[1] = pos

	eng := &mockEngine{}
	pf := NewPriceFeed(s, eng)

	pf.UpdatePrice(1, 50000)

	if eng.closePositionCalled {
		t.Error("Expected ClosePosition to not be called when position is healthy")
	}
}

func TestUpdatePrice_TriggersLiquidation(t *testing.T) {
	s := &state.State{
		Users: map[types.UserID]*state.UserState{
			1: {
				Balances:  make(map[string]*types.UserBalance),
				Positions: make(map[types.SymbolID]*types.Position),
			},
		},
		Symbols: make(map[types.SymbolID]*state.OrderBookState),
	}

	pos := &types.Position{
		UserID:            1,
		Symbol:            1,
		Size:              10,
		Side:              constants.ORDER_SIDE_BUY,
		EntryPrice:        50000,
		Leverage:          10,
		InitialMargin:     5000000,
		MaintenanceMargin: 500000,
	}
	position.CalculatePositionRisk(pos)

	s.Users[1].Positions[1] = pos

	eng := &mockEngine{}
	pf := NewPriceFeed(s, eng)

	buffer := pos.InitialMargin - pos.MaintenanceMargin
	liqPrice := types.Price(int64(pos.EntryPrice) - int64(buffer)/int64(pos.Size) - 100)

	pf.UpdatePrice(1, liqPrice)

	if !eng.closePositionCalled {
		t.Error("Expected ClosePosition to be called when position is liquidated")
	}
	if eng.lastUserID != 1 {
		t.Errorf("Expected lastUserID to be 1, got %d", eng.lastUserID)
	}
	if eng.lastSymbol != 1 {
		t.Errorf("Expected lastSymbol to be 1, got %d", eng.lastSymbol)
	}
}

func TestUpdatePrice_DifferentSymbol(t *testing.T) {
	s := &state.State{
		Users: map[types.UserID]*state.UserState{
			1: {
				Balances:  make(map[string]*types.UserBalance),
				Positions: make(map[types.SymbolID]*types.Position),
			},
		},
		Symbols: make(map[types.SymbolID]*state.OrderBookState),
	}

	pos := &types.Position{
		UserID:            1,
		Symbol:            2,
		Size:              10,
		Side:              constants.ORDER_SIDE_BUY,
		EntryPrice:        3000,
		Leverage:          10,
		InitialMargin:     300000,
		MaintenanceMargin: 30000,
	}
	position.CalculatePositionRisk(pos)

	s.Users[1].Positions[2] = pos

	eng := &mockEngine{}
	pf := NewPriceFeed(s, eng)

	pf.UpdatePrice(1, 50000)

	if eng.closePositionCalled {
		t.Error("Expected ClosePosition to not be called when updating different symbol")
	}
}

func TestGetPrice(t *testing.T) {
	s := &state.State{
		Users:   make(map[types.UserID]*state.UserState),
		Symbols: make(map[types.SymbolID]*state.OrderBookState),
	}
	eng := &mockEngine{}
	pf := NewPriceFeed(s, eng)

	pf.UpdatePrice(1, 50000)

	price := pf.GetPrice(1)
	if price != 50000 {
		t.Errorf("Expected price to be 50000, got %d", price)
	}
}

func TestGetPrice_NotSet(t *testing.T) {
	s := &state.State{
		Users:   make(map[types.UserID]*state.UserState),
		Symbols: make(map[types.SymbolID]*state.OrderBookState),
	}
	eng := &mockEngine{}
	pf := NewPriceFeed(s, eng)

	price := pf.GetPrice(1)
	if price != 0 {
		t.Errorf("Expected price to be 0 for unset symbol, got %d", price)
	}
}
