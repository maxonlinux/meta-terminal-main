package engine

import (
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/position"
	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func TestEditLeverage_SuccessIncrease(t *testing.T) {
	s := state.New()
	e := New(nil, s)

	userID := types.UserID(1)
	symbol := types.SymbolID(1)

	us := s.GetUserState(userID)
	us.Balances["USDT"] = &types.UserBalance{
		UserID:    userID,
		Asset:     "USDT",
		Available: 2500000,
		Locked:    0,
		Margin:    0,
	}

	us.Positions[symbol] = &types.Position{
		UserID:      userID,
		Symbol:      symbol,
		Size:        100,
		Side:        0,
		EntryPrice:  50000,
		Leverage:    2,
		RealizedPnl: 0,
	}

	bal := us.Balances["USDT"]
	oldMargin := int64(100) * int64(50000) / int64(2)
	bal.Margin = oldMargin

	err := e.EditLeverage(userID, symbol, 10)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	pos := us.Positions[symbol]
	if pos.Leverage != 10 {
		t.Errorf("expected leverage 10, got %d", pos.Leverage)
	}

	newMargin := int64(100) * int64(50000) / int64(10)
	if bal.Margin != newMargin {
		t.Errorf("expected margin %d, got %d", newMargin, bal.Margin)
	}

	releasedMargin := oldMargin - newMargin
	expectedAvailable := int64(2500000) + releasedMargin
	if bal.Available != expectedAvailable {
		t.Errorf("expected available %d, got %d", expectedAvailable, bal.Available)
	}
}

func TestEditLeverage_SuccessDecrease(t *testing.T) {
	s := state.New()
	e := New(nil, s)

	userID := types.UserID(1)
	symbol := types.SymbolID(1)

	us := s.GetUserState(userID)
	us.Balances["USDT"] = &types.UserBalance{
		UserID:    userID,
		Asset:     "USDT",
		Available: 2500000,
		Locked:    0,
		Margin:    0,
	}

	us.Positions[symbol] = &types.Position{
		UserID:      userID,
		Symbol:      symbol,
		Size:        100,
		Side:        0,
		EntryPrice:  50000,
		Leverage:    10,
		RealizedPnl: 0,
	}

	bal := us.Balances["USDT"]
	oldMargin := int64(100) * int64(50000) / int64(10)
	bal.Margin = oldMargin

	err := e.EditLeverage(userID, symbol, 2)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	pos := us.Positions[symbol]
	if pos.Leverage != 2 {
		t.Errorf("expected leverage 2, got %d", pos.Leverage)
	}

	newMargin := int64(100) * int64(50000) / int64(2)
	if bal.Margin != newMargin {
		t.Errorf("expected margin %d, got %d", newMargin, bal.Margin)
	}

	requiredMargin := newMargin - oldMargin
	expectedAvailable := int64(2500000) - requiredMargin
	if bal.Available != expectedAvailable {
		t.Errorf("expected available %d, got %d", expectedAvailable, bal.Available)
	}
}

func TestEditLeverage_InsufficientBalance(t *testing.T) {
	s := state.New()
	e := New(nil, s)

	userID := types.UserID(1)
	symbol := types.SymbolID(1)

	us := s.GetUserState(userID)
	us.Balances["USDT"] = &types.UserBalance{
		UserID:    userID,
		Asset:     "USDT",
		Available: 100000,
		Locked:    0,
		Margin:    0,
	}

	us.Positions[symbol] = &types.Position{
		UserID:      userID,
		Symbol:      symbol,
		Size:        100,
		Side:        0,
		EntryPrice:  50000,
		Leverage:    10,
		RealizedPnl: 0,
	}

	bal := us.Balances["USDT"]
	bal.Margin = int64(100) * int64(50000) / int64(10)

	err := e.EditLeverage(userID, symbol, 2)

	if err != ErrInsufficientBalance {
		t.Errorf("expected ErrInsufficientBalance, got %v", err)
	}

	pos := us.Positions[symbol]
	if pos.Leverage != 10 {
		t.Errorf("leverage should not change on error, expected 10, got %d", pos.Leverage)
	}
}

func TestEditLeverage_InvalidLeverage(t *testing.T) {
	s := state.New()
	e := New(nil, s)

	userID := types.UserID(1)
	symbol := types.SymbolID(1)

	us := s.GetUserState(userID)
	us.Balances["USDT"] = &types.UserBalance{
		UserID:    userID,
		Asset:     "USDT",
		Available: 100000,
		Locked:    0,
		Margin:    0,
	}

	us.Positions[symbol] = &types.Position{
		UserID:      userID,
		Symbol:      symbol,
		Size:        100,
		Side:        0,
		EntryPrice:  50000,
		Leverage:    2,
		RealizedPnl: 0,
	}

	err := e.EditLeverage(userID, symbol, 0)
	if err != ErrInvalidLeverage {
		t.Errorf("expected ErrInvalidLeverage for 0, got %v", err)
	}

	err = e.EditLeverage(userID, symbol, 101)
	if err != ErrInvalidLeverage {
		t.Errorf("expected ErrInvalidLeverage for 101, got %v", err)
	}
}

func TestEditLeverage_NoPosition(t *testing.T) {
	s := state.New()
	e := New(nil, s)

	userID := types.UserID(1)
	symbol := types.SymbolID(1)

	us := s.GetUserState(userID)
	us.Balances["USDT"] = &types.UserBalance{
		UserID:    userID,
		Asset:     "USDT",
		Available: 100000,
		Locked:    0,
		Margin:    0,
	}

	err := e.EditLeverage(userID, symbol, 20)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	pos := us.Positions[symbol]
	if pos == nil {
		t.Fatal("position should be created")
	}
	if pos.Leverage != 20 {
		t.Errorf("expected leverage 20, got %d", pos.Leverage)
	}
	if pos.Size != 0 {
		t.Errorf("expected size 0, got %d", pos.Size)
	}
	if pos.Side != -1 {
		t.Errorf("expected side -1 (null), got %d", pos.Side)
	}
}

func TestEditLeverage_SameLeverage(t *testing.T) {
	s := state.New()
	e := New(nil, s)

	userID := types.UserID(1)
	symbol := types.SymbolID(1)

	us := s.GetUserState(userID)
	us.Balances["USDT"] = &types.UserBalance{
		UserID:    userID,
		Asset:     "USDT",
		Available: 2500000,
		Locked:    0,
		Margin:    2500000,
	}

	us.Positions[symbol] = &types.Position{
		UserID:      userID,
		Symbol:      symbol,
		Size:        100,
		Side:        0,
		EntryPrice:  50000,
		Leverage:    2,
		RealizedPnl: 0,
	}

	initialAvailable := us.Balances["USDT"].Available
	initialMargin := us.Balances["USDT"].Margin

	err := e.EditLeverage(userID, symbol, 2)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if us.Balances["USDT"].Available != initialAvailable {
		t.Errorf("available should not change, expected %d, got %d", initialAvailable, us.Balances["USDT"].Available)
	}
	if us.Balances["USDT"].Margin != initialMargin {
		t.Errorf("margin should not change, expected %d, got %d", initialMargin, us.Balances["USDT"].Margin)
	}
}

func TestEditLeverage_EmptyPositionUpdatesLeverage(t *testing.T) {
	s := state.New()
	e := New(nil, s)

	userID := types.UserID(1)
	symbol := types.SymbolID(1)

	us := s.GetUserState(userID)
	us.Balances["USDT"] = &types.UserBalance{
		UserID:    userID,
		Asset:     "USDT",
		Available: 100000,
		Locked:    0,
		Margin:    0,
	}

	us.Positions[symbol] = &types.Position{
		UserID:      userID,
		Symbol:      symbol,
		Size:        0,
		Side:        -1,
		EntryPrice:  0,
		Leverage:    2,
		RealizedPnl: 0,
	}

	err := e.EditLeverage(userID, symbol, 10)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	pos := us.Positions[symbol]
	if pos.Leverage != 10 {
		t.Errorf("expected leverage 10, got %d", pos.Leverage)
	}
	if pos.Size != 0 {
		t.Errorf("size should remain 0, got %d", pos.Size)
	}
	if pos.Side != -1 {
		t.Errorf("side should remain -1, got %d", pos.Side)
	}
}

func TestPosition_SideSetOnOpen(t *testing.T) {
	s := state.New()
	_ = New(nil, s)

	userID := types.UserID(1)
	symbol := types.SymbolID(1)

	us := s.GetUserState(userID)
	us.Balances["USDT"] = &types.UserBalance{
		UserID:    userID,
		Asset:     "USDT",
		Available: 10000000,
		Locked:    0,
		Margin:    0,
	}

	position.UpdatePosition(s, userID, symbol, 100, 50000, 0, 2)

	pos := us.Positions[symbol]
	if pos.Side != 0 {
		t.Errorf("expected side 0 (BUY/LONG), got %d", pos.Side)
	}
	if pos.Size != 100 {
		t.Errorf("expected size 100, got %d", pos.Size)
	}
	if pos.Leverage != 2 {
		t.Errorf("expected leverage 2, got %d", pos.Leverage)
	}
}

func TestPosition_SideReversed(t *testing.T) {
	s := state.New()
	_ = New(nil, s)

	userID := types.UserID(1)
	symbol := types.SymbolID(1)

	us := s.GetUserState(userID)
	us.Balances["USDT"] = &types.UserBalance{
		UserID:    userID,
		Asset:     "USDT",
		Available: 10000000,
		Locked:    0,
		Margin:    0,
	}

	position.UpdatePosition(s, userID, symbol, 100, 50000, 0, 2)
	position.UpdatePosition(s, userID, symbol, 150, 51000, 1, 2)

	pos := us.Positions[symbol]
	if pos.Side != 1 {
		t.Errorf("expected side 1 (SELL/SHORT), got %d", pos.Side)
	}
	if pos.Size != 50 {
		t.Errorf("expected size 50, got %d", pos.Size)
	}
}

func TestPosition_SideNullOnClose(t *testing.T) {
	s := state.New()
	_ = New(nil, s)

	userID := types.UserID(1)
	symbol := types.SymbolID(1)

	us := s.GetUserState(userID)
	us.Balances["USDT"] = &types.UserBalance{
		UserID:    userID,
		Asset:     "USDT",
		Available: 10000000,
		Locked:    0,
		Margin:    0,
	}

	position.UpdatePosition(s, userID, symbol, 100, 50000, 0, 2)
	position.UpdatePosition(s, userID, symbol, 100, 51000, 1, 2)

	pos := us.Positions[symbol]
	if pos.Side != -1 {
		t.Errorf("expected side -1 (null), got %d", pos.Side)
	}
	if pos.Size != 0 {
		t.Errorf("expected size 0, got %d", pos.Size)
	}
}
