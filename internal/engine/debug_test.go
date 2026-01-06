package engine

import (
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/internal/types"
	"github.com/anomalyco/meta-terminal-go/internal/wal"
)

func TestDebugOrderMatching(t *testing.T) {
	tmpDir := t.TempDir()
	st := state.New()
	w, _ := wal.New(tmpDir+"/wal", 64)
	e := New(w, st)

	// Initialize symbol as LINEAR
	e.InitSymbolCategory(1, constants.CATEGORY_LINEAR)

	// Seed balance for user 1
	us1 := st.GetUserState(1)
	us1.Balances["USDT"] = &types.UserBalance{
		UserID:    1,
		Asset:     "USDT",
		Available: 1000000,
		Locked:    0,
		Margin:    0,
		Version:   0,
	}

	// User 1 places LIMIT BUY @ 50000
	input1 := &types.OrderInput{
		UserID:   1,
		Symbol:   1,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 10,
		Price:    50000,
	}
	result1, err1 := e.PlaceOrder(input1)
	t.Logf("LIMIT BUY: err=%v, status=%d, orderID=%d", err1, result1.Status, result1.Order.ID)

	// Check order book
	bids, asks := e.GetOrderBook(1, 10)
	t.Logf("OrderBook - Bids: %v, Asks: %v", bids, asks)

	// Seed balance for user 2
	us2 := st.GetUserState(2)
	us2.Balances["USDT"] = &types.UserBalance{
		UserID:    2,
		Asset:     "USDT",
		Available: 1000000,
		Locked:    0,
		Margin:    0,
		Version:   0,
	}

	// User 2 places MARKET SELL
	input2 := &types.OrderInput{
		UserID:   2,
		Symbol:   1,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_MARKET,
		TIF:      constants.TIF_GTC,
		Quantity: 10,
		Price:    0,
	}
	result2, err2 := e.PlaceOrder(input2)
	t.Logf("MARKET SELL: err=%v, status=%d, filled=%d", err2, result2.Status, result2.Filled)

	// Check positions
	pos1 := e.GetUserPosition(1, 1)
	pos2 := e.GetUserPosition(2, 1)
	t.Logf("User1 Position: %+v", pos1)
	t.Logf("User2 Position: %+v", pos2)

	// Check order book again
	bids, asks = e.GetOrderBook(1, 10)
	t.Logf("OrderBook after - Bids: %v, Asks: %v", bids, asks)
}

func TestDebugIOCPartialFill(t *testing.T) {
	tmpDir := t.TempDir()
	st := state.New()
	w, _ := wal.New(tmpDir+"/wal", 64)
	e := New(w, st)

	// Initialize symbol as LINEAR
	e.InitSymbolCategory(1, constants.CATEGORY_LINEAR)

	// Seed balance for user 1
	us1 := st.GetUserState(1)
	us1.Balances["USDT"] = &types.UserBalance{
		UserID:    1,
		Asset:     "USDT",
		Available: 1000000,
		Locked:    0,
		Margin:    0,
		Version:   0,
	}

	// User 1 places LIMIT BUY 5 @ 50000
	input1 := &types.OrderInput{
		UserID:   1,
		Symbol:   1,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 5,
		Price:    50000,
	}
	result1, err1 := e.PlaceOrder(input1)
	t.Logf("LIMIT BUY 5: err=%v, status=%d, order.Quantity=%d, order.Filled=%d",
		err1, result1.Status, result1.Order.Quantity, result1.Order.Filled)

	// Check order book
	bids, asks := e.GetOrderBook(1, 10)
	t.Logf("OrderBook after LIMIT: Bids: %v, Asks: %v", bids, asks)

	// Seed balance for user 2
	us2 := st.GetUserState(2)
	us2.Balances["USDT"] = &types.UserBalance{
		UserID:    2,
		Asset:     "USDT",
		Available: 1000000,
		Locked:    0,
		Margin:    0,
		Version:   0,
	}

	// User 2 places MARKET SELL 10 IOC
	input2 := &types.OrderInput{
		UserID:   2,
		Symbol:   1,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_MARKET,
		TIF:      constants.TIF_IOC,
		Quantity: 10,
		Price:    0,
	}
	result2, err2 := e.PlaceOrder(input2)
	t.Logf("MARKET SELL 10 IOC: err=%v, status=%d (FILLED=%d, PARTIAL=%d), filled=%d, remaining=%d",
		err2, result2.Status, constants.ORDER_STATUS_FILLED, constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED,
		result2.Filled, result2.Remaining)

	// Also check the order object directly
	t.Logf("Order object: ID=%d, Quantity=%d, Filled=%d, Status=%d",
		result2.Order.ID, result2.Order.Quantity, result2.Order.Filled, result2.Order.Status)

	// Check positions
	pos1 := e.GetUserPosition(1, 1)
	pos2 := e.GetUserPosition(2, 1)
	t.Logf("User1 Position: %+v", pos1)
	t.Logf("User2 Position: %+v", pos2)

	// Check order book
	bids, asks = e.GetOrderBook(1, 10)
	t.Logf("OrderBook after IOC: Bids: %v, Asks: %v", bids, asks)

	// Expected: status=4 (PARTIAL_CANCELED), filled=5
	if result2.Status != 4 {
		t.Errorf("expected status 4 (PARTIAL_CANCELED), got %d", result2.Status)
	}
	if result2.Filled != 5 {
		t.Errorf("expected filled 5, got %d", result2.Filled)
	}
}

func TestDebugReduceOnly(t *testing.T) {
	tmpDir := t.TempDir()
	st := state.New()
	w, _ := wal.New(tmpDir+"/wal", 64)
	e := New(w, st)

	// Initialize symbol as LINEAR
	e.InitSymbolCategory(1, constants.CATEGORY_LINEAR)

	// Seed balances
	us1 := st.GetUserState(1)
	us1.Balances["USDT"] = &types.UserBalance{
		UserID:    1,
		Asset:     "USDT",
		Available: 1000000,
		Locked:    0,
		Margin:    0,
		Version:   0,
	}
	us2 := st.GetUserState(2)
	us2.Balances["USDT"] = &types.UserBalance{
		UserID:    2,
		Asset:     "USDT",
		Available: 1000000,
		Locked:    0,
		Margin:    0,
		Version:   0,
	}

	// User 2 places LIMIT SELL 10 @ 50000
	input1 := &types.OrderInput{
		UserID:   2,
		Symbol:   1,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 10,
		Price:    50000,
	}
	result1, err1 := e.PlaceOrder(input1)
	t.Logf("LIMIT SELL 10: err=%v, status=%d", err1, result1.Status)

	// Check order book
	bids, asks := e.GetOrderBook(1, 10)
	t.Logf("OrderBook after LIMIT SELL: Bids: %v, Asks: %v", bids, asks)

	// User 1 places MARKET BUY 10
	input2 := &types.OrderInput{
		UserID:   1,
		Symbol:   1,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_MARKET,
		TIF:      constants.TIF_GTC,
		Quantity: 10,
		Price:    0,
	}
	result2, err2 := e.PlaceOrder(input2)
	t.Logf("MARKET BUY 10: err=%v, status=%d, filled=%d", err2, result2.Status, result2.Filled)

	// Check position
	pos1 := e.GetUserPosition(1, 1)
	t.Logf("User1 Position after BUY: %+v", pos1)

	// Check order book
	bids, asks = e.GetOrderBook(1, 10)
	t.Logf("OrderBook after MARKET BUY: Bids: %v, Asks: %v", bids, asks)

	// Edit leverage
	err := e.EditLeverage(1, 1, 10)
	t.Logf("EditLeverage result: err=%v", err)

	// Check position after leverage change
	pos1 = e.GetUserPosition(1, 1)
	t.Logf("User1 Position after EditLeverage: %+v", pos1)
}

func TestDebugEditLeverageEmptyPosition(t *testing.T) {
	tmpDir := t.TempDir()
	st := state.New()
	w, _ := wal.New(tmpDir+"/wal", 64)
	e := New(w, st)

	// Initialize symbol as LINEAR
	e.InitSymbolCategory(1, constants.CATEGORY_LINEAR)

	// Seed balance
	us1 := st.GetUserState(1)
	us1.Balances["USDT"] = &types.UserBalance{
		UserID:    1,
		Asset:     "USDT",
		Available: 1000000,
		Locked:    0,
		Margin:    0,
		Version:   0,
	}

	// Check position before EditLeverage
	pos := e.GetUserPosition(1, 1)
	t.Logf("Position before EditLeverage: %+v", pos)

	// Edit leverage on empty position
	err := e.EditLeverage(1, 1, 10)
	t.Logf("EditLeverage on empty position: err=%v", err)

	// Check position after EditLeverage
	pos = e.GetUserPosition(1, 1)
	t.Logf("Position after EditLeverage: %+v", pos)

	if pos == nil {
		t.Fatalf("position should exist after EditLeverage")
	}
	if pos.Leverage != 10 {
		t.Errorf("expected leverage 10, got %d", pos.Leverage)
	}
	if pos.Size != 0 {
		t.Errorf("expected size 0 (empty position), got %d", pos.Size)
	}
}

func TestDebugFOKFullFill(t *testing.T) {
	tmpDir := t.TempDir()
	st := state.New()
	w, _ := wal.New(tmpDir+"/wal", 64)
	e := New(w, st)

	// Initialize symbol as LINEAR
	e.InitSymbolCategory(1, constants.CATEGORY_LINEAR)

	// Seed balances
	us1 := st.GetUserState(1)
	us1.Balances["USDT"] = &types.UserBalance{
		UserID:    1,
		Asset:     "USDT",
		Available: 1000000,
		Locked:    0,
		Margin:    0,
		Version:   0,
	}
	us2 := st.GetUserState(2)
	us2.Balances["USDT"] = &types.UserBalance{
		UserID:    2,
		Asset:     "USDT",
		Available: 1000000,
		Locked:    0,
		Margin:    0,
		Version:   0,
	}

	// User 2 provides 10 BTC @ 50000
	input1 := &types.OrderInput{
		UserID:   2,
		Symbol:   1,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 10,
		Price:    50000,
	}
	e.PlaceOrder(input1)

	// User 1 places FOK BUY 10 - should fill completely
	input2 := &types.OrderInput{
		UserID:   1,
		Symbol:   1,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_MARKET,
		TIF:      constants.TIF_FOK,
		Quantity: 10,
		Price:    0,
	}
	result, err := e.PlaceOrder(input2)
	t.Logf("FOK BUY 10 (full fill): err=%v, status=%d, filled=%d", err, result.Status, result.Filled)

	if result.Status != constants.ORDER_STATUS_FILLED {
		t.Errorf("expected FILLED, got %d", result.Status)
	}
	if result.Filled != 10 {
		t.Errorf("expected filled 10, got %d", result.Filled)
	}

	pos1 := e.GetUserPosition(1, 1)
	pos2 := e.GetUserPosition(2, 1)
	t.Logf("User1 Position: %+v", pos1)
	t.Logf("User2 Position: %+v", pos2)
}

func TestDebugFOKPartialFill(t *testing.T) {
	tmpDir := t.TempDir()
	st := state.New()
	w, _ := wal.New(tmpDir+"/wal", 64)
	e := New(w, st)

	// Initialize symbol as LINEAR
	e.InitSymbolCategory(1, constants.CATEGORY_LINEAR)

	// Seed balances
	us1 := st.GetUserState(1)
	us1.Balances["USDT"] = &types.UserBalance{
		UserID:    1,
		Asset:     "USDT",
		Available: 1000000,
		Locked:    0,
		Margin:    0,
		Version:   0,
	}
	us2 := st.GetUserState(2)
	us2.Balances["USDT"] = &types.UserBalance{
		UserID:    2,
		Asset:     "USDT",
		Available: 1000000,
		Locked:    0,
		Margin:    0,
		Version:   0,
	}

	t.Logf("Test FOKPartialFill starting...")
	bids0, asks0 := e.GetOrderBook(1, 10)
	t.Logf("OrderBook before: Bids: %v, Asks: %v", bids0, asks0)

	// User 2 provides only 5 BTC @ 50000
	input1 := &types.OrderInput{
		UserID:   2,
		Symbol:   1,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 5,
		Price:    50000,
	}
	result1, _ := e.PlaceOrder(input1)
	t.Logf("LIMIT SELL 5: result=%d, order.Quantity=%d", result1.Status, result1.Order.Quantity)

	// Check order book
	bids, asks := e.GetOrderBook(1, 10)
	t.Logf("OrderBook after LIMIT SELL 5: Bids: %v, Asks: %v", bids, asks)

	// Check symbol state directly
	ss := st.GetSymbolState(1)
	t.Logf("SymbolState Asks level: Price=%d, Quantity=%d, FirstOrderID=%d",
		ss.Asks.Price, ss.Asks.Quantity, ss.Asks.FirstOrderID)

	// Check available quantity
	available := e.getAvailableQuantity(1, 0, 0) // BUY side checks Asks, MARKET price=0
	t.Logf("Available quantity for BUY: %d", available)

	// User 1 places FOK BUY 10 - should be CANCELED (not enough liquidity)
	input2 := &types.OrderInput{
		UserID:   1,
		Symbol:   1,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_MARKET,
		TIF:      constants.TIF_FOK,
		Quantity: 10,
		Price:    0,
	}
	result, err := e.PlaceOrder(input2)
	t.Logf("FOK BUY 10 (partial fill): err=%v, status=%d, filled=%d", err, result.Status, result.Filled)

	if result.Status != constants.ORDER_STATUS_CANCELED {
		t.Errorf("expected CANCELED, got %d", result.Status)
	}
	if result.Filled != 0 {
		t.Errorf("expected filled 0, got %d", result.Filled)
	}

	// Position should not exist
	pos1 := e.GetUserPosition(1, 1)
	if pos1 != nil {
		t.Errorf("expected no position, got %v", pos1)
	}

	// User 2 should still have their order in book (fills are NOT lost for maker)
	bids, asks = e.GetOrderBook(1, 10)
	t.Logf("OrderBook after FOK rejection: Bids: %v, Asks: %v", bids, asks)
}

func TestDebugPOST_ONLY_Rejected(t *testing.T) {
	tmpDir := t.TempDir()
	st := state.New()
	w, _ := wal.New(tmpDir+"/wal", 64)
	e := New(w, st)

	e.InitSymbolCategory(1, constants.CATEGORY_LINEAR)

	us1 := st.GetUserState(1)
	us1.Balances["USDT"] = &types.UserBalance{
		UserID:    1,
		Asset:     "USDT",
		Available: 1000000,
		Locked:    0,
		Margin:    0,
		Version:   0,
	}

	// User 1 places LIMIT SELL @ 50000 (creates ask at 50000)
	input1 := &types.OrderInput{
		UserID:   1,
		Symbol:   1,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 10,
		Price:    50000,
	}
	result1, _ := e.PlaceOrder(input1)
	t.Logf("LIMIT SELL 10 @ 50000: status=%d", result1.Status)

	bids, asks := e.GetOrderBook(1, 10)
	t.Logf("OrderBook: Bids: %v, Asks: %v", bids, asks)

	// User 2 places POST_ONLY BUY @ 50001 - should be REJECTED (crosses spread)
	input2 := &types.OrderInput{
		UserID:   2,
		Symbol:   1,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_POST_ONLY,
		Quantity: 10,
		Price:    50001,
	}
	result2, err := e.PlaceOrder(input2)
	t.Logf("POST_ONLY BUY 10 @ 50001: err=%v, status=%d", err, result2.Status)

	// Should be CANCELED (rejected)
	if result2.Status != constants.ORDER_STATUS_CANCELED {
		t.Errorf("expected CANCELED, got %d", result2.Status)
	}

	// Order book should be unchanged
	bids, asks = e.GetOrderBook(1, 10)
	t.Logf("OrderBook after POST_ONLY rejection: Bids: %v, Asks: %v", bids, asks)
	if len(asks) != 1 || asks[0][0] != 50000 || asks[0][1] != 10 {
		t.Errorf("ask book changed unexpectedly: %v", asks)
	}
}

func TestDebugPOST_ONLY_Accepted(t *testing.T) {
	tmpDir := t.TempDir()
	st := state.New()
	w, _ := wal.New(tmpDir+"/wal", 64)
	e := New(w, st)

	e.InitSymbolCategory(1, constants.CATEGORY_LINEAR)

	us1 := st.GetUserState(1)
	us1.Balances["USDT"] = &types.UserBalance{
		UserID:    1,
		Asset:     "USDT",
		Available: 1000000,
		Locked:    0,
		Margin:    0,
		Version:   0,
	}

	// User 1 places LIMIT SELL @ 50000 (creates ask at 50000)
	input1 := &types.OrderInput{
		UserID:   1,
		Symbol:   1,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 10,
		Price:    50000,
	}
	result1, _ := e.PlaceOrder(input1)
	t.Logf("LIMIT SELL 10 @ 50000: status=%d", result1.Status)

	bids, asks := e.GetOrderBook(1, 10)
	t.Logf("OrderBook: Bids: %v, Asks: %v", bids, asks)

	// User 2 places POST_ONLY BUY @ 49999 - should be ACCEPTED (below ask)
	input2 := &types.OrderInput{
		UserID:   2,
		Symbol:   1,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_POST_ONLY,
		Quantity: 10,
		Price:    49999,
	}
	result2, err := e.PlaceOrder(input2)
	t.Logf("POST_ONLY BUY 10 @ 49999: err=%v, status=%d", err, result2.Status)

	// Should be NEW (resting in book)
	if result2.Status != constants.ORDER_STATUS_NEW {
		t.Errorf("expected NEW, got %d", result2.Status)
	}

	// Should be in order book as bid
	bids, asks = e.GetOrderBook(1, 10)
	t.Logf("OrderBook after POST_ONLY: Bids: %v, Asks: %v", bids, asks)
	if len(bids) != 1 || bids[0][0] != 49999 || bids[0][1] != 10 {
		t.Errorf("bid not in book: %v", bids)
	}
}

func TestDebugPOST_ONLY_ExactSpread(t *testing.T) {
	tmpDir := t.TempDir()
	st := state.New()
	w, _ := wal.New(tmpDir+"/wal", 64)
	e := New(w, st)

	e.InitSymbolCategory(1, constants.CATEGORY_LINEAR)

	us1 := st.GetUserState(1)
	us1.Balances["USDT"] = &types.UserBalance{
		UserID:    1,
		Asset:     "USDT",
		Available: 1000000,
		Locked:    0,
		Margin:    0,
		Version:   0,
	}

	// User 1 places LIMIT SELL @ 50000 (creates ask at 50000)
	input1 := &types.OrderInput{
		UserID:   1,
		Symbol:   1,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 10,
		Price:    50000,
	}
	result1, _ := e.PlaceOrder(input1)
	t.Logf("LIMIT SELL 10 @ 50000: status=%d", result1.Status)

	bids, asks := e.GetOrderBook(1, 10)
	t.Logf("OrderBook: Bids: %v, Asks: %v", bids, asks)

	// User 2 places POST_ONLY BUY @ 50000 (at ask price - does NOT cross)
	input2 := &types.OrderInput{
		UserID:   2,
		Symbol:   1,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_POST_ONLY,
		Quantity: 10,
		Price:    50000,
	}
	result2, err := e.PlaceOrder(input2)
	t.Logf("POST_ONLY BUY 10 @ 50000 (at ask): err=%v, status=%d", err, result2.Status)

	// Should be NEW (resting in book - doesn't cross spread)
	if result2.Status != constants.ORDER_STATUS_NEW {
		t.Errorf("expected NEW, got %d", result2.Status)
	}

	// Should be in order book as bid at same price
	bids, asks = e.GetOrderBook(1, 10)
	t.Logf("OrderBook after POST_ONLY: Bids: %v, Asks: %v", bids, asks)
	if len(bids) != 1 || len(asks) != 1 {
		t.Errorf("expected both bids and asks: %v, %v", bids, asks)
	}
}
