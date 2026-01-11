package oms

import (
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

type countingClearing struct {
	reserveCalls int
	lastQty      types.Quantity
	lastPrice    types.Price
	lastSide     int8
	lastCategory int8
	lastSymbol   string
}

func (c *countingClearing) Reserve(userID types.UserID, symbol string, category int8, side int8, qty types.Quantity, price types.Price) error {
	c.reserveCalls++
	c.lastQty = qty
	c.lastPrice = price
	c.lastSide = side
	c.lastCategory = category
	c.lastSymbol = symbol
	return nil
}

func (c *countingClearing) Release(userID types.UserID, symbol string, category int8, side int8, qty types.Quantity, price types.Price) {
}

func (c *countingClearing) ExecuteTrade(trade *types.Trade, taker *types.Order, maker *types.Order) {
}

func newTestService() (*Service, *testPortfolio) {
	portfolio := &testPortfolio{
		positions: make(map[types.UserID]map[string]*types.Position),
	}
	clearing := &testClearing{}

	s, _ := New(Config{}, portfolio, clearing)
	return s, portfolio
}

func newTestServiceWithClearing(clearing Clearing) (*Service, *testPortfolio) {
	portfolio := &testPortfolio{
		positions: make(map[types.UserID]map[string]*types.Position),
	}
	s, _ := New(Config{}, portfolio, clearing)
	return s, portfolio
}
