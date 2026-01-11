package clearing

import (
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/portfolio"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

type mockPortfolio struct {
	lastAsset  string
	lastAmount int64
	pos        *types.Position
}

func (m *mockPortfolio) Reserve(userID types.UserID, asset string, amount int64) error {
	m.lastAsset = asset
	m.lastAmount = amount
	return nil
}

func (m *mockPortfolio) Release(userID types.UserID, asset string, amount int64) {
	m.lastAsset = asset
	m.lastAmount = amount
}

func (m *mockPortfolio) ExecuteTrade(trade *types.Trade, taker, maker *types.Order) {}

func (m *mockPortfolio) GetPositions(userID types.UserID) []*types.Position { return nil }

func (m *mockPortfolio) GetPosition(userID types.UserID, symbol string) *types.Position {
	if m.pos != nil {
		return m.pos
	}
	return &types.Position{Symbol: symbol, Size: 0, Side: -1}
}

func TestReserve_SpotBuy(t *testing.T) {
	p := &mockPortfolio{}
	s := New(p)

	_ = s.Reserve(1, "BTCUSDT", constants.CATEGORY_SPOT, constants.ORDER_SIDE_BUY, 2, 100)
	if p.lastAsset != "USDT" {
		t.Fatalf("expected USDT asset, got %s", p.lastAsset)
	}
	if p.lastAmount != 200 {
		t.Fatalf("expected amount 200, got %d", p.lastAmount)
	}
}

func TestReserve_SpotSell(t *testing.T) {
	p := &mockPortfolio{}
	s := New(p)

	_ = s.Reserve(1, "BTCUSDT", constants.CATEGORY_SPOT, constants.ORDER_SIDE_SELL, 3, 100)
	if p.lastAsset != "BTC" {
		t.Fatalf("expected BTC asset, got %s", p.lastAsset)
	}
	if p.lastAmount != 3 {
		t.Fatalf("expected amount 3, got %d", p.lastAmount)
	}
}

func TestReserve_LinearUsesLeverage(t *testing.T) {
	p := &mockPortfolio{
		pos: &types.Position{Symbol: "BTCUSDT", Leverage: 5},
	}
	s := New(p)

	_ = s.Reserve(1, "BTCUSDT", constants.CATEGORY_LINEAR, constants.ORDER_SIDE_BUY, 10, 100)
	if p.lastAsset != "USDT" {
		t.Fatalf("expected USDT asset, got %s", p.lastAsset)
	}
	if p.lastAmount != 200 {
		t.Fatalf("expected amount 200, got %d", p.lastAmount)
	}
}

func TestReserve_LinearDefaultLeverage(t *testing.T) {
	p := &mockPortfolio{
		pos: &types.Position{Symbol: "BTCUSDT", Leverage: 0},
	}
	s := New(p)

	_ = s.Reserve(1, "BTCUSDT", constants.CATEGORY_LINEAR, constants.ORDER_SIDE_BUY, 4, 50)
	if p.lastAmount != 100 {
		t.Fatalf("expected amount 100 with default leverage, got %d", p.lastAmount)
	}
}

func TestExecuteTrade_SpotFlow(t *testing.T) {
	port := portfolio.New(portfolio.Config{})
	s := New(port)

	takerID := types.UserID(1)
	makerID := types.UserID(2)

	port.Balances[takerID] = map[string]*types.UserBalance{
		"USDT": {Asset: "USDT", Available: 50000, Locked: 50000},
		"BTC":  {Asset: "BTC", Available: 0, Locked: 0},
	}
	port.Balances[makerID] = map[string]*types.UserBalance{
		"USDT": {Asset: "USDT", Available: 0, Locked: 0},
		"BTC":  {Asset: "BTC", Available: 0, Locked: 1},
	}

	trade := &types.Trade{
		ID:           1,
		Symbol:       "BTCUSDT",
		Category:     constants.CATEGORY_SPOT,
		TakerID:      takerID,
		MakerID:      makerID,
		TakerOrderID: 10,
		MakerOrderID: 11,
		Price:        50000,
		Quantity:     1,
		ExecutedAt:   1,
	}
	taker := &types.Order{
		ID:       10,
		UserID:   takerID,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_SPOT,
		Side:     constants.ORDER_SIDE_BUY,
		Price:    50000,
		Quantity: 1,
	}
	maker := &types.Order{
		ID:       11,
		UserID:   makerID,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_SPOT,
		Side:     constants.ORDER_SIDE_SELL,
		Price:    50000,
		Quantity: 1,
	}

	s.ExecuteTrade(trade, taker, maker)

	if got := port.Balances[takerID]["USDT"]; got.Locked != 0 || got.Available != 50000 {
		t.Fatalf("taker USDT unexpected: %+v", got)
	}
	if got := port.Balances[takerID]["BTC"]; got.Available != 1 {
		t.Fatalf("taker BTC expected 1, got %+v", got)
	}
	if got := port.Balances[makerID]["BTC"]; got.Locked != 0 || got.Available != 0 {
		t.Fatalf("maker BTC unexpected: %+v", got)
	}
	if got := port.Balances[makerID]["USDT"]; got.Available != 50000 {
		t.Fatalf("maker USDT expected 50000, got %+v", got)
	}
}

func TestExecuteTrade_LinearFlow(t *testing.T) {
	port := portfolio.New(portfolio.Config{})
	s := New(port)

	takerID := types.UserID(1)
	makerID := types.UserID(2)

	port.Balances[takerID] = map[string]*types.UserBalance{
		"USDT": {Asset: "USDT", Available: 0, Locked: 10000, Margin: 0},
	}
	port.Balances[makerID] = map[string]*types.UserBalance{
		"USDT": {Asset: "USDT", Available: 0, Locked: 10000, Margin: 0},
	}
	port.Positions[takerID] = map[string]*types.Position{
		"BTCUSDT": {Symbol: "BTCUSDT", Size: 0, Side: -1, Leverage: 5},
	}
	port.Positions[makerID] = map[string]*types.Position{
		"BTCUSDT": {Symbol: "BTCUSDT", Size: 0, Side: -1, Leverage: 5},
	}

	trade := &types.Trade{
		ID:           1,
		Symbol:       "BTCUSDT",
		Category:     constants.CATEGORY_LINEAR,
		TakerID:      takerID,
		MakerID:      makerID,
		TakerOrderID: 10,
		MakerOrderID: 11,
		Price:        50000,
		Quantity:     1,
		ExecutedAt:   1,
	}
	taker := &types.Order{
		ID:       10,
		UserID:   takerID,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Price:    50000,
		Quantity: 1,
	}
	maker := &types.Order{
		ID:       11,
		UserID:   makerID,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_SELL,
		Price:    50000,
		Quantity: 1,
	}

	s.ExecuteTrade(trade, taker, maker)

	if got := port.Balances[takerID]["USDT"]; got.Locked != 0 || got.Margin != 10000 {
		t.Fatalf("taker USDT expected locked 0 margin 10000, got %+v", got)
	}
	if got := port.Balances[makerID]["USDT"]; got.Locked != 0 || got.Margin != 10000 {
		t.Fatalf("maker USDT expected locked 0 margin 10000, got %+v", got)
	}
	if pos := port.Positions[takerID]["BTCUSDT"]; pos.Size != 1 || pos.Side != constants.ORDER_SIDE_BUY {
		t.Fatalf("taker position unexpected: %+v", pos)
	}
	if pos := port.Positions[makerID]["BTCUSDT"]; pos.Size != -1 || pos.Side != constants.ORDER_SIDE_SELL {
		t.Fatalf("maker position unexpected: %+v", pos)
	}
}

func TestExecuteTrade_SpotPartialFills(t *testing.T) {
	port := portfolio.New(portfolio.Config{})
	s := New(port)

	takerID := types.UserID(1)
	makerID := types.UserID(2)

	port.Balances[takerID] = map[string]*types.UserBalance{
		"USDT": {Asset: "USDT", Available: 0, Locked: 100000},
		"BTC":  {Asset: "BTC", Available: 0, Locked: 0},
	}
	port.Balances[makerID] = map[string]*types.UserBalance{
		"USDT": {Asset: "USDT", Available: 0, Locked: 0},
		"BTC":  {Asset: "BTC", Available: 0, Locked: 2},
	}

	taker := &types.Order{
		ID:       10,
		UserID:   takerID,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_SPOT,
		Side:     constants.ORDER_SIDE_BUY,
		Price:    50000,
		Quantity: 2,
	}
	maker := &types.Order{
		ID:       11,
		UserID:   makerID,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_SPOT,
		Side:     constants.ORDER_SIDE_SELL,
		Price:    50000,
		Quantity: 2,
	}

	trade1 := &types.Trade{
		ID:           1,
		Symbol:       "BTCUSDT",
		Category:     constants.CATEGORY_SPOT,
		TakerID:      takerID,
		MakerID:      makerID,
		TakerOrderID: 10,
		MakerOrderID: 11,
		Price:        50000,
		Quantity:     1,
		ExecutedAt:   1,
	}
	trade2 := &types.Trade{
		ID:           2,
		Symbol:       "BTCUSDT",
		Category:     constants.CATEGORY_SPOT,
		TakerID:      takerID,
		MakerID:      makerID,
		TakerOrderID: 10,
		MakerOrderID: 11,
		Price:        50000,
		Quantity:     1,
		ExecutedAt:   2,
	}

	s.ExecuteTrade(trade1, taker, maker)
	s.ExecuteTrade(trade2, taker, maker)

	if got := port.Balances[takerID]["USDT"]; got.Locked != 0 || got.Available != 0 {
		t.Fatalf("taker USDT expected locked 0 available 0, got %+v", got)
	}
	if got := port.Balances[takerID]["BTC"]; got.Available != 2 {
		t.Fatalf("taker BTC expected 2, got %+v", got)
	}
	if got := port.Balances[makerID]["BTC"]; got.Locked != 0 || got.Available != 0 {
		t.Fatalf("maker BTC expected 0, got %+v", got)
	}
	if got := port.Balances[makerID]["USDT"]; got.Available != 100000 {
		t.Fatalf("maker USDT expected 100000, got %+v", got)
	}
}

func TestExecuteTrade_LinearPartialFills(t *testing.T) {
	port := portfolio.New(portfolio.Config{})
	s := New(port)

	takerID := types.UserID(1)
	makerID := types.UserID(2)

	port.Balances[takerID] = map[string]*types.UserBalance{
		"USDT": {Asset: "USDT", Available: 0, Locked: 20000, Margin: 0},
	}
	port.Balances[makerID] = map[string]*types.UserBalance{
		"USDT": {Asset: "USDT", Available: 0, Locked: 20000, Margin: 0},
	}
	port.Positions[takerID] = map[string]*types.Position{
		"BTCUSDT": {Symbol: "BTCUSDT", Size: 0, Side: -1, Leverage: 5},
	}
	port.Positions[makerID] = map[string]*types.Position{
		"BTCUSDT": {Symbol: "BTCUSDT", Size: 0, Side: -1, Leverage: 5},
	}

	taker := &types.Order{
		ID:       10,
		UserID:   takerID,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Price:    50000,
		Quantity: 2,
	}
	maker := &types.Order{
		ID:       11,
		UserID:   makerID,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_SELL,
		Price:    50000,
		Quantity: 2,
	}

	trade1 := &types.Trade{
		ID:           1,
		Symbol:       "BTCUSDT",
		Category:     constants.CATEGORY_LINEAR,
		TakerID:      takerID,
		MakerID:      makerID,
		TakerOrderID: 10,
		MakerOrderID: 11,
		Price:        50000,
		Quantity:     1,
		ExecutedAt:   1,
	}
	trade2 := &types.Trade{
		ID:           2,
		Symbol:       "BTCUSDT",
		Category:     constants.CATEGORY_LINEAR,
		TakerID:      takerID,
		MakerID:      makerID,
		TakerOrderID: 10,
		MakerOrderID: 11,
		Price:        50000,
		Quantity:     1,
		ExecutedAt:   2,
	}

	s.ExecuteTrade(trade1, taker, maker)
	s.ExecuteTrade(trade2, taker, maker)

	if got := port.Balances[takerID]["USDT"]; got.Locked != 0 || got.Margin != 20000 {
		t.Fatalf("taker USDT expected locked 0 margin 20000, got %+v", got)
	}
	if got := port.Balances[makerID]["USDT"]; got.Locked != 0 || got.Margin != 20000 {
		t.Fatalf("maker USDT expected locked 0 margin 20000, got %+v", got)
	}
	if pos := port.Positions[takerID]["BTCUSDT"]; pos.Size != 2 || pos.Side != constants.ORDER_SIDE_BUY {
		t.Fatalf("taker position unexpected: %+v", pos)
	}
	if pos := port.Positions[makerID]["BTCUSDT"]; pos.Size != -2 || pos.Side != constants.ORDER_SIDE_SELL {
		t.Fatalf("maker position unexpected: %+v", pos)
	}
}
