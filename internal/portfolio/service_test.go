package portfolio

import (
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func TestSpotTrade_BalanceFlow(t *testing.T) {
	s := New(Config{})

	userID := types.UserID(1)
	s.Balances[userID] = make(map[string]*types.UserBalance)
	s.Balances[userID]["BTC"] = &types.UserBalance{Asset: "BTC", Available: 100, Locked: 0, Margin: 0}
	s.Balances[userID]["USDT"] = &types.UserBalance{Asset: "USDT", Available: 50000, Locked: 0, Margin: 0}

	trade := &types.Trade{
		ID:           1,
		Symbol:       "BTCUSDT",
		Category:     constants.CATEGORY_SPOT,
		TakerID:      userID,
		MakerID:      2,
		TakerOrderID: 100,
		MakerOrderID: 101,
		Price:        50000,
		Quantity:     1,
		ExecutedAt:   1234567890,
	}

	taker := &types.Order{
		ID:       100,
		UserID:   userID,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_SPOT,
		Side:     constants.ORDER_SIDE_BUY,
		Price:    50000,
		Quantity: 1,
	}

	maker := &types.Order{
		ID:       101,
		UserID:   2,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_SPOT,
		Side:     constants.ORDER_SIDE_SELL,
		Price:    50000,
		Quantity: 1,
	}

	s.ExecuteTrade(trade, taker, maker)

	if s.Balances[userID]["USDT"].Locked != 0 {
		t.Errorf("USDT Locked = %d, want 0", s.Balances[userID]["USDT"].Locked)
	}
	if s.Balances[userID]["USDT"].Available != 0 {
		t.Errorf("USDT Available = %d, want 0 (all used for trade)", s.Balances[userID]["USDT"].Available)
	}
	if s.Balances[userID]["BTC"].Locked != 0 {
		t.Errorf("BTC Locked = %d, want 0", s.Balances[userID]["BTC"].Locked)
	}
	if s.Balances[userID]["BTC"].Available != 101 {
		t.Errorf("BTC Available = %d, want 101", s.Balances[userID]["BTC"].Available)
	}
	if s.Positions[userID] != nil {
		t.Errorf("SPOT trade should NOT create position, but positions map exists")
	}
}

func TestLinearTrade_PositionAndMargin(t *testing.T) {
	s := New(Config{})

	userID := types.UserID(1)
	s.Balances[userID] = make(map[string]*types.UserBalance)
	s.Balances[userID]["USDT"] = &types.UserBalance{Asset: "USDT", Available: 100000, Locked: 0, Margin: 0}

	trade := &types.Trade{
		ID:           1,
		Symbol:       "BTCUSDT",
		Category:     constants.CATEGORY_LINEAR,
		TakerID:      userID,
		MakerID:      2,
		TakerOrderID: 100,
		MakerOrderID: 101,
		Price:        50000,
		Quantity:     1,
		ExecutedAt:   1234567890,
	}

	taker := &types.Order{
		ID:       100,
		UserID:   userID,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Price:    50000,
		Quantity: 1,
	}

	maker := &types.Order{
		ID:       101,
		UserID:   2,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_SELL,
		Price:    50000,
		Quantity: 1,
	}

	s.ExecuteTrade(trade, taker, maker)

	margin := int64(50000) * int64(1) / int64(2)
	if s.Balances[userID]["USDT"].Margin != margin {
		t.Errorf("USDT Margin = %d, want %d", s.Balances[userID]["USDT"].Margin, margin)
	}

	pos := s.Positions[userID]["BTCUSDT"]
	if pos == nil {
		t.Fatal("LINEAR trade should create position")
	}
	if pos.Size != 1 {
		t.Errorf("Position Size = %d, want 1", pos.Size)
	}
	if pos.Side != constants.ORDER_SIDE_BUY {
		t.Errorf("Position Side = %d, want %d (BUY)", pos.Side, constants.ORDER_SIDE_BUY)
	}
	if pos.EntryPrice != 50000 {
		t.Errorf("Position EntryPrice = %d, want 50000", pos.EntryPrice)
	}
	if pos.Leverage != 2 {
		t.Errorf("Position Leverage = %d, want 2", pos.Leverage)
	}
}

func TestGetPosition_NoPosition(t *testing.T) {
	s := New(Config{})

	pos := s.GetPosition(1, "BTCUSDT")

	if pos.Symbol != "BTCUSDT" {
		t.Errorf("Position Symbol = %s, want BTCUSDT", pos.Symbol)
	}
	if pos.Size != 0 {
		t.Errorf("Position Size = %d, want 0", pos.Size)
	}
	if pos.Side != -1 {
		t.Errorf("Position Side = %d, want -1 (NONE)", pos.Side)
	}
}

func BenchmarkSpotTrade(b *testing.B) {
	b.ReportAllocs()
	s := New(Config{})
	userID := types.UserID(1)
	s.Balances[userID] = make(map[string]*types.UserBalance)
	s.Balances[userID]["BTC"] = &types.UserBalance{Asset: "BTC", Available: 100, Locked: 0, Margin: 0}
	s.Balances[userID]["USDT"] = &types.UserBalance{Asset: "USDT", Available: 50000, Locked: 0, Margin: 0}

	trade := &types.Trade{
		ID:           1,
		Symbol:       "BTCUSDT",
		Category:     constants.CATEGORY_SPOT,
		TakerID:      userID,
		MakerID:      2,
		TakerOrderID: 100,
		MakerOrderID: 101,
		Price:        50000,
		Quantity:     1,
		ExecutedAt:   1234567890,
	}

	taker := &types.Order{
		ID:       100,
		UserID:   userID,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_SPOT,
		Side:     constants.ORDER_SIDE_BUY,
		Price:    50000,
		Quantity: 1,
	}

	maker := &types.Order{
		ID:       101,
		UserID:   2,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_SPOT,
		Side:     constants.ORDER_SIDE_SELL,
		Price:    50000,
		Quantity: 1,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.ExecuteTrade(trade, taker, maker)
	}
}

func BenchmarkLinearTrade(b *testing.B) {
	b.ReportAllocs()
	s := New(Config{})
	userID := types.UserID(1)
	s.Balances[userID] = make(map[string]*types.UserBalance)
	s.Balances[userID]["USDT"] = &types.UserBalance{Asset: "USDT", Available: 100000, Locked: 0, Margin: 0}

	trade := &types.Trade{
		ID:           1,
		Symbol:       "BTCUSDT",
		Category:     constants.CATEGORY_LINEAR,
		TakerID:      userID,
		MakerID:      2,
		TakerOrderID: 100,
		MakerOrderID: 101,
		Price:        50000,
		Quantity:     1,
		ExecutedAt:   1234567890,
	}

	taker := &types.Order{
		ID:       100,
		UserID:   userID,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Price:    50000,
		Quantity: 1,
	}

	maker := &types.Order{
		ID:       101,
		UserID:   2,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_SELL,
		Price:    50000,
		Quantity: 1,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.ExecuteTrade(trade, taker, maker)
	}
}

func TestSpotTrade_Conservation(t *testing.T) {
	s := New(Config{})

	userA := types.UserID(1)
	userB := types.UserID(2)

	s.Balances[userA] = map[string]*types.UserBalance{
		"BTC":  {Asset: "BTC", Available: 5, Locked: 0, Margin: 0},
		"USDT": {Asset: "USDT", Available: 100000, Locked: 0, Margin: 0},
	}
	s.Balances[userB] = map[string]*types.UserBalance{
		"BTC":  {Asset: "BTC", Available: 3, Locked: 0, Margin: 0},
		"USDT": {Asset: "USDT", Available: 50000, Locked: 0, Margin: 0},
	}

	baseTotalBefore := s.Balances[userA]["BTC"].Available + s.Balances[userB]["BTC"].Available
	quoteTotalBefore := s.Balances[userA]["USDT"].Available + s.Balances[userB]["USDT"].Available

	trades := []*types.Trade{
		{ID: 1, Symbol: "BTCUSDT", Category: constants.CATEGORY_SPOT, TakerID: userA, MakerID: userB, TakerOrderID: 10, MakerOrderID: 11, Price: 10000, Quantity: 1},
		{ID: 2, Symbol: "BTCUSDT", Category: constants.CATEGORY_SPOT, TakerID: userB, MakerID: userA, TakerOrderID: 12, MakerOrderID: 13, Price: 12000, Quantity: 2},
		{ID: 3, Symbol: "BTCUSDT", Category: constants.CATEGORY_SPOT, TakerID: userA, MakerID: userB, TakerOrderID: 14, MakerOrderID: 15, Price: 9000, Quantity: 1},
	}

	for _, trade := range trades {
		taker := &types.Order{
			ID:       trade.TakerOrderID,
			UserID:   trade.TakerID,
			Symbol:   trade.Symbol,
			Category: trade.Category,
			Side:     constants.ORDER_SIDE_BUY,
			Price:    trade.Price,
			Quantity: trade.Quantity,
		}
		maker := &types.Order{
			ID:       trade.MakerOrderID,
			UserID:   trade.MakerID,
			Symbol:   trade.Symbol,
			Category: trade.Category,
			Side:     constants.ORDER_SIDE_SELL,
			Price:    trade.Price,
			Quantity: trade.Quantity,
		}
		s.ExecuteTrade(trade, taker, maker)
	}

	baseTotalAfter := s.Balances[userA]["BTC"].Available + s.Balances[userB]["BTC"].Available
	quoteTotalAfter := s.Balances[userA]["USDT"].Available + s.Balances[userB]["USDT"].Available

	if baseTotalBefore != baseTotalAfter {
		t.Fatalf("base total changed: before %d after %d", baseTotalBefore, baseTotalAfter)
	}
	if quoteTotalBefore != quoteTotalAfter {
		t.Fatalf("quote total changed: before %d after %d", quoteTotalBefore, quoteTotalAfter)
	}
}

func TestLinearTrade_Conservation(t *testing.T) {
	s := New(Config{})

	userA := types.UserID(1)
	userB := types.UserID(2)

	s.Balances[userA] = map[string]*types.UserBalance{
		"USDT": {Asset: "USDT", Available: 100000, Locked: 0, Margin: 0},
	}
	s.Balances[userB] = map[string]*types.UserBalance{
		"USDT": {Asset: "USDT", Available: 80000, Locked: 0, Margin: 0},
	}
	s.Positions[userA] = map[string]*types.Position{
		"BTCUSDT": {Symbol: "BTCUSDT", Size: 0, Side: -1, Leverage: 5},
	}
	s.Positions[userB] = map[string]*types.Position{
		"BTCUSDT": {Symbol: "BTCUSDT", Size: 0, Side: -1, Leverage: 5},
	}

	totalBefore := sumUSDT(s.Balances[userA]) + sumUSDT(s.Balances[userB])

	trade := &types.Trade{
		ID:           1,
		Symbol:       "BTCUSDT",
		Category:     constants.CATEGORY_LINEAR,
		TakerID:      userA,
		MakerID:      userB,
		TakerOrderID: 10,
		MakerOrderID: 11,
		Price:        10000,
		Quantity:     2,
	}
	taker := &types.Order{
		ID:       10,
		UserID:   userA,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Price:    trade.Price,
		Quantity: trade.Quantity,
	}
	maker := &types.Order{
		ID:       11,
		UserID:   userB,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_SELL,
		Price:    trade.Price,
		Quantity: trade.Quantity,
	}

	s.ExecuteTrade(trade, taker, maker)

	totalAfter := sumUSDT(s.Balances[userA]) + sumUSDT(s.Balances[userB])
	if totalBefore != totalAfter {
		t.Fatalf("USDT total changed: before %d after %d", totalBefore, totalAfter)
	}
}

func sumUSDT(balances map[string]*types.UserBalance) int64 {
	if balances == nil {
		return 0
	}
	if b := balances["USDT"]; b != nil {
		return b.Available + b.Locked + b.Margin
	}
	return 0
}

func TestNoNegativeBalancesAfterTrades(t *testing.T) {
	s := New(Config{})

	userA := types.UserID(1)
	userB := types.UserID(2)

	s.Balances[userA] = map[string]*types.UserBalance{
		"BTC":  {Asset: "BTC", Available: 5, Locked: 0, Margin: 0},
		"USDT": {Asset: "USDT", Available: 95000, Locked: 5000, Margin: 0},
	}
	s.Balances[userB] = map[string]*types.UserBalance{
		"BTC":  {Asset: "BTC", Available: 5, Locked: 0, Margin: 0},
		"USDT": {Asset: "USDT", Available: 95000, Locked: 5000, Margin: 0},
	}
	s.Positions[userA] = map[string]*types.Position{
		"BTCUSDT": {Symbol: "BTCUSDT", Size: 0, Side: -1, Leverage: 5},
	}
	s.Positions[userB] = map[string]*types.Position{
		"BTCUSDT": {Symbol: "BTCUSDT", Size: 0, Side: -1, Leverage: 5},
	}

	trades := []struct {
		trade *types.Trade
		taker *types.Order
		maker *types.Order
	}{
		{
			trade: &types.Trade{ID: 1, Symbol: "BTCUSDT", Category: constants.CATEGORY_SPOT, TakerID: userA, MakerID: userB, Price: 10000, Quantity: 1},
			taker: &types.Order{ID: 10, UserID: userA, Symbol: "BTCUSDT", Category: constants.CATEGORY_SPOT, Side: constants.ORDER_SIDE_BUY, Price: 10000, Quantity: 1},
			maker: &types.Order{ID: 11, UserID: userB, Symbol: "BTCUSDT", Category: constants.CATEGORY_SPOT, Side: constants.ORDER_SIDE_SELL, Price: 10000, Quantity: 1},
		},
		{
			trade: &types.Trade{ID: 2, Symbol: "BTCUSDT", Category: constants.CATEGORY_SPOT, TakerID: userB, MakerID: userA, Price: 9000, Quantity: 2},
			taker: &types.Order{ID: 12, UserID: userB, Symbol: "BTCUSDT", Category: constants.CATEGORY_SPOT, Side: constants.ORDER_SIDE_BUY, Price: 9000, Quantity: 2},
			maker: &types.Order{ID: 13, UserID: userA, Symbol: "BTCUSDT", Category: constants.CATEGORY_SPOT, Side: constants.ORDER_SIDE_SELL, Price: 9000, Quantity: 2},
		},
		{
			trade: &types.Trade{ID: 3, Symbol: "BTCUSDT", Category: constants.CATEGORY_LINEAR, TakerID: userA, MakerID: userB, Price: 12000, Quantity: 1},
			taker: &types.Order{ID: 14, UserID: userA, Symbol: "BTCUSDT", Category: constants.CATEGORY_LINEAR, Side: constants.ORDER_SIDE_BUY, Price: 12000, Quantity: 1},
			maker: &types.Order{ID: 15, UserID: userB, Symbol: "BTCUSDT", Category: constants.CATEGORY_LINEAR, Side: constants.ORDER_SIDE_SELL, Price: 12000, Quantity: 1},
		},
		{
			trade: &types.Trade{ID: 4, Symbol: "BTCUSDT", Category: constants.CATEGORY_LINEAR, TakerID: userB, MakerID: userA, Price: 11000, Quantity: 1},
			taker: &types.Order{ID: 16, UserID: userB, Symbol: "BTCUSDT", Category: constants.CATEGORY_LINEAR, Side: constants.ORDER_SIDE_BUY, Price: 11000, Quantity: 1},
			maker: &types.Order{ID: 17, UserID: userA, Symbol: "BTCUSDT", Category: constants.CATEGORY_LINEAR, Side: constants.ORDER_SIDE_SELL, Price: 11000, Quantity: 1},
		},
	}

	for _, item := range trades {
		s.ExecuteTrade(item.trade, item.taker, item.maker)
		assertNoNegative(t, s.Balances[userA])
		assertNoNegative(t, s.Balances[userB])
	}
}

func assertNoNegative(t *testing.T, balances map[string]*types.UserBalance) {
	t.Helper()
	for asset, bal := range balances {
		if bal.Available < 0 || bal.Locked < 0 || bal.Margin < 0 {
			t.Fatalf("negative balance for %s: %+v", asset, bal)
		}
	}
}

func TestSetLeverage_NewPosition(t *testing.T) {
	s := New(Config{})

	userID := types.UserID(1)
	s.Balances[userID] = make(map[string]*types.UserBalance)
	s.Balances[userID]["USDT"] = &types.UserBalance{Asset: "USDT", Available: 100000, Locked: 0, Margin: 0}

	err := s.SetLeverage(userID, "BTCUSDT", 10, 50000)
	if err != nil {
		t.Errorf("SetLeverage for new position failed: %v", err)
	}

	pos := s.GetPosition(userID, "BTCUSDT")
	if pos.Leverage != 10 {
		t.Errorf("Position leverage = %d, want 10", pos.Leverage)
	}
}

func TestSetLeverage_ExistingPosition_IncreaseLeverage(t *testing.T) {
	s := New(Config{})

	userID := types.UserID(1)
	s.Balances[userID] = make(map[string]*types.UserBalance)
	s.Balances[userID]["USDT"] = &types.UserBalance{Asset: "USDT", Available: 80000, Locked: 0, Margin: 25000}

	s.Positions[userID] = make(map[string]*types.Position)
	s.Positions[userID]["BTCUSDT"] = &types.Position{
		Symbol:     "BTCUSDT",
		Size:       1,
		Side:       constants.ORDER_SIDE_BUY,
		EntryPrice: 50000,
		Leverage:   2,
	}

	err := s.SetLeverage(userID, "BTCUSDT", 10, 20000)
	if err != nil {
		t.Errorf("SetLeverage increase failed: %v", err)
	}

	pos := s.Positions[userID]["BTCUSDT"]
	if pos.Leverage != 10 {
		t.Errorf("Position leverage = %d, want 10", pos.Leverage)
	}

	margin := int64(50000) * 1 / 10
	if s.Balances[userID]["USDT"].Margin != margin {
		t.Errorf("Margin = %d, want %d", s.Balances[userID]["USDT"].Margin, margin)
	}
	if s.Balances[userID]["USDT"].Available != 80000+(25000-margin) {
		t.Errorf("Available = %d, want %d", s.Balances[userID]["USDT"].Available, 80000+(25000-margin))
	}
}

func TestSetLeverage_ExistingPosition_DecreaseLeverage(t *testing.T) {
	s := New(Config{})

	userID := types.UserID(1)
	s.Balances[userID] = make(map[string]*types.UserBalance)
	s.Balances[userID]["USDT"] = &types.UserBalance{Asset: "USDT", Available: 30000, Locked: 0, Margin: 5000}

	s.Positions[userID] = make(map[string]*types.Position)
	s.Positions[userID]["BTCUSDT"] = &types.Position{
		Symbol:     "BTCUSDT",
		Size:       1,
		Side:       constants.ORDER_SIDE_BUY,
		EntryPrice: 50000,
		Leverage:   10,
	}

	err := s.SetLeverage(userID, "BTCUSDT", 2, 20000)
	if err != nil {
		t.Errorf("SetLeverage decrease failed: %v", err)
	}

	pos := s.Positions[userID]["BTCUSDT"]
	if pos.Leverage != 2 {
		t.Errorf("Position leverage = %d, want 2", pos.Leverage)
	}

	margin := int64(50000) * 1 / 2
	if s.Balances[userID]["USDT"].Margin != margin {
		t.Errorf("Margin = %d, want %d", s.Balances[userID]["USDT"].Margin, margin)
	}
	if s.Balances[userID]["USDT"].Available != 30000+(5000-margin) {
		t.Errorf("Available = %d, want %d", s.Balances[userID]["USDT"].Available, 30000+(5000-margin))
	}
}

func TestSetLeverage_InsuffientBalance(t *testing.T) {
	s := New(Config{})

	userID := types.UserID(1)
	s.Balances[userID] = make(map[string]*types.UserBalance)
	s.Balances[userID]["USDT"] = &types.UserBalance{Asset: "USDT", Available: 5000, Locked: 0, Margin: 25000}

	s.Positions[userID] = make(map[string]*types.Position)
	s.Positions[userID]["BTCUSDT"] = &types.Position{
		Symbol:     "BTCUSDT",
		Size:       1,
		Side:       constants.ORDER_SIDE_BUY,
		EntryPrice: 50000,
		Leverage:   2,
	}

	err := s.SetLeverage(userID, "BTCUSDT", 10, 20000)
	if err != ErrInsufficientBalance {
		t.Errorf("Expected ErrInsufficientBalance, got %v", err)
	}
}

func TestSetLeverage_WouldLiquidate(t *testing.T) {
	s := New(Config{})

	userID := types.UserID(1)
	s.Balances[userID] = make(map[string]*types.UserBalance)
	s.Balances[userID]["USDT"] = &types.UserBalance{Asset: "USDT", Available: 100000, Locked: 0, Margin: 5000}

	s.Positions[userID] = make(map[string]*types.Position)
	s.Positions[userID]["BTCUSDT"] = &types.Position{
		Symbol:     "BTCUSDT",
		Size:       1,
		Side:       constants.ORDER_SIDE_BUY,
		EntryPrice: 50000,
		Leverage:   2,
	}

	err := s.SetLeverage(userID, "BTCUSDT", 11, 50000)
	if err != ErrLeverageTooHigh {
		t.Errorf("Expected ErrLeverageTooHigh, got %v", err)
	}
}

func TestSetLeverage_DefaultLeverage(t *testing.T) {
	s := New(Config{})

	userID := types.UserID(1)

	err := s.SetLeverage(userID, "BTCUSDT", 0, 50000)
	if err != nil {
		t.Errorf("SetLeverage with 0 should use default: %v", err)
	}

	pos := s.GetPosition(userID, "BTCUSDT")
	if pos.Leverage != constants.DEFAULT_LEVERAGE {
		t.Errorf("Position leverage = %d, want %d (default)", pos.Leverage, constants.DEFAULT_LEVERAGE)
	}
}

func TestSetLeverage_MaxLeverage(t *testing.T) {
	s := New(Config{})

	userID := types.UserID(1)

	err := s.SetLeverage(userID, "BTCUSDT", 50, 50000)
	if err != nil {
		t.Errorf("SetLeverage with 50 should cap at 20: %v", err)
	}

	pos := s.GetPosition(userID, "BTCUSDT")
	if pos.Leverage != 20 {
		t.Errorf("Position leverage = %d, want 20 (max)", pos.Leverage)
	}
}

func BenchmarkSetLeverage(b *testing.B) {
	b.ReportAllocs()
	s := New(Config{})
	userID := types.UserID(1)
	s.Balances[userID] = make(map[string]*types.UserBalance)
	s.Balances[userID]["USDT"] = &types.UserBalance{Asset: "USDT", Available: 1000000, Locked: 0, Margin: 50000}
	s.Positions[userID] = make(map[string]*types.Position)
	s.Positions[userID]["BTCUSDT"] = &types.Position{
		Symbol:     "BTCUSDT",
		Size:       10,
		Side:       constants.ORDER_SIDE_BUY,
		EntryPrice: 50000,
		Leverage:   2,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.SetLeverage(userID, "BTCUSDT", 5, 50000)
	}
}
