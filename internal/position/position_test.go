package position

import (
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func TestUpdatePositionOpenLong(t *testing.T) {
	s := state.New()
	userID := types.UserID(1)
	symbol := types.SymbolID(1)

	pos, pnl := UpdatePosition(s, userID, symbol, 10, 100, constants.ORDER_SIDE_BUY)

	if pos.Size != 10 {
		t.Errorf("expected size 10, got %d", pos.Size)
	}
	if pos.EntryPrice != 100 {
		t.Errorf("expected entry price 100, got %d", pos.EntryPrice)
	}
	if pnl != 0 {
		t.Errorf("expected pnl 0, got %d", pnl)
	}
}

func TestUpdatePositionOpenShort(t *testing.T) {
	s := state.New()
	userID := types.UserID(1)
	symbol := types.SymbolID(1)

	pos, pnl := UpdatePosition(s, userID, symbol, 10, 100, constants.ORDER_SIDE_SELL)

	if pos.Size != 10 {
		t.Errorf("expected size 10, got %d", pos.Size)
	}
	if pos.Side != constants.ORDER_SIDE_SELL {
		t.Errorf("expected side SELL (1), got %d", pos.Side)
	}
	if pos.EntryPrice != 100 {
		t.Errorf("expected entry price 100, got %d", pos.EntryPrice)
	}
	if pnl != 0 {
		t.Errorf("expected pnl 0, got %d", pnl)
	}
}

func TestUpdatePositionAddToLong(t *testing.T) {
	s := state.New()
	userID := types.UserID(1)
	symbol := types.SymbolID(1)

	UpdatePosition(s, userID, symbol, 10, 100, constants.ORDER_SIDE_BUY)
	pos, pnl := UpdatePosition(s, userID, symbol, 5, 110, constants.ORDER_SIDE_BUY)

	if pos.Size != 15 {
		t.Errorf("expected size 15, got %d", pos.Size)
	}
	if pnl != 0 {
		t.Errorf("expected pnl 0, got %d", pnl)
	}
}

func TestUpdatePositionCloseLong(t *testing.T) {
	s := state.New()
	userID := types.UserID(1)
	symbol := types.SymbolID(1)

	UpdatePosition(s, userID, symbol, 10, 100, constants.ORDER_SIDE_BUY)
	pos, pnl := UpdatePosition(s, userID, symbol, 5, 120, constants.ORDER_SIDE_SELL)

	if pos.Size != 5 {
		t.Errorf("expected size 5, got %d", pos.Size)
	}
	if pnl != 100 {
		t.Errorf("expected pnl 100, got %d", pnl)
	}
}

func TestUpdatePositionCloseFully(t *testing.T) {
	s := state.New()
	userID := types.UserID(1)
	symbol := types.SymbolID(1)

	UpdatePosition(s, userID, symbol, 10, 100, constants.ORDER_SIDE_BUY)
	pos, pnl := UpdatePosition(s, userID, symbol, 10, 120, constants.ORDER_SIDE_SELL)

	if pos.Size != 0 {
		t.Errorf("expected size 0, got %d", pos.Size)
	}
	if pos.EntryPrice != 0 {
		t.Errorf("expected entry price 0, got %d", pos.EntryPrice)
	}
	if pnl != 200 {
		t.Errorf("expected pnl 200, got %d", pnl)
	}
}

func TestUpdatePositionReverseShort(t *testing.T) {
	s := state.New()
	userID := types.UserID(1)
	symbol := types.SymbolID(1)

	UpdatePosition(s, userID, symbol, 10, 100, constants.ORDER_SIDE_SELL)
	pos, pnl := UpdatePosition(s, userID, symbol, 15, 90, constants.ORDER_SIDE_BUY)

	if pos.Size != 5 {
		t.Errorf("expected size 5, got %d", pos.Size)
	}
	if pnl != 100 {
		t.Errorf("expected pnl 100, got %d", pnl)
	}
}

func TestGetPosition(t *testing.T) {
	s := state.New()
	userID := types.UserID(1)
	symbol := types.SymbolID(1)

	pos := GetPosition(s, userID, symbol)
	if pos != nil {
		t.Error("expected nil position")
	}

	UpdatePosition(s, userID, symbol, 10, 100, constants.ORDER_SIDE_BUY)

	pos = GetPosition(s, userID, symbol)
	if pos == nil {
		t.Fatal("expected position, got nil")
	}
	if pos.Size != 10 {
		t.Errorf("expected size 10, got %d", pos.Size)
	}
}

func TestReduceOnlyValidate(t *testing.T) {
	s := state.New()
	userID := types.UserID(1)
	symbol := types.SymbolID(1)

	valid := ReduceOnlyValidate(s, userID, symbol, 5, constants.ORDER_SIDE_SELL)
	if valid {
		t.Error("expected false with no position")
	}

	UpdatePosition(s, userID, symbol, 10, 100, constants.ORDER_SIDE_BUY)

	valid = ReduceOnlyValidate(s, userID, symbol, 5, constants.ORDER_SIDE_BUY)
	if valid {
		t.Error("expected false for BUY reduceOnly")
	}

	valid = ReduceOnlyValidate(s, userID, symbol, 5, constants.ORDER_SIDE_SELL)
	if !valid {
		t.Error("expected true for SELL reduceOnly within position")
	}

	valid = ReduceOnlyValidate(s, userID, symbol, 15, constants.ORDER_SIDE_SELL)
	if valid {
		t.Error("expected false for reduceOnly exceeding position")
	}
}

func TestAdjustReduceOnlyOrdersCancel(t *testing.T) {
	s := state.New()
	userID := types.UserID(1)
	symbol := types.SymbolID(1)

	us := s.GetUserState(userID)
	us.Positions[symbol] = &types.Position{
		UserID:     userID,
		Symbol:     symbol,
		Size:       10,
		EntryPrice: 100,
	}

	ss := s.GetSymbolState(symbol)
	ss.OrderMap = make(map[types.OrderID]*types.Order)
	ss.UserReduceOnly = make(map[types.UserID][]types.OrderID)

	order1 := &types.Order{
		ID:         1,
		UserID:     userID,
		Symbol:     symbol,
		ReduceOnly: true,
		Quantity:   5,
		Filled:     0,
		Status:     constants.ORDER_STATUS_NEW,
	}
	order2 := &types.Order{
		ID:         2,
		UserID:     userID,
		Symbol:     symbol,
		ReduceOnly: true,
		Quantity:   10,
		Filled:     0,
		Status:     constants.ORDER_STATUS_NEW,
	}

	ss.OrderMap[1] = order1
	ss.OrderMap[2] = order2
	ss.UserReduceOnly[userID] = []types.OrderID{1, 2}

	AdjustReduceOnlyOrders(s, userID, symbol)

	if order1.Status != constants.ORDER_STATUS_CANCELED {
		t.Errorf("expected order1 canceled, got status %d", order1.Status)
	}
	if order2.Quantity != 10 {
		t.Errorf("expected order2 quantity 10, got %d", order2.Quantity)
	}
}

func TestAbs(t *testing.T) {
	if abs(10) != 10 {
		t.Errorf("abs(10) = 10, got %d", abs(10))
	}
	if abs(-10) != 10 {
		t.Errorf("abs(-10) = 10, got %d", abs(-10))
	}
}

func TestMin(t *testing.T) {
	if min(5, 10) != 5 {
		t.Errorf("min(5, 10) = 5, got %d", min(5, 10))
	}
	if min(10, 5) != 5 {
		t.Errorf("min(10, 5) = 5, got %d", min(10, 5))
	}
}
