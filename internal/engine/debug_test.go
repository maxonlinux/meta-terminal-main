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
