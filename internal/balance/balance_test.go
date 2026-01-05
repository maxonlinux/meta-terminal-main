package balance

import (
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func TestLock(t *testing.T) {
	s := state.New()
	userID := types.UserID(1)

	us := s.GetUserState(userID)
	us.Balances["USDT"] = &types.UserBalance{
		UserID:    userID,
		Asset:     "USDT",
		Available: 1000,
	}

	err := Lock(s, userID, "USDT", 1000)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	balance := us.Balances["USDT"]
	if balance.Available != 0 {
		t.Errorf("expected available 0, got %d", balance.Available)
	}
	if balance.Locked != 1000 {
		t.Errorf("expected locked 1000, got %d", balance.Locked)
	}
}

func TestLockInsufficientBalance(t *testing.T) {
	s := state.New()
	userID := types.UserID(1)

	err := Lock(s, userID, "USDT", 1000)
	if err != ErrInsufficientBalance {
		t.Errorf("expected ErrInsufficientBalance, got %v", err)
	}
}

func TestUnlock(t *testing.T) {
	s := state.New()
	userID := types.UserID(1)

	us := s.GetUserState(userID)
	us.Balances["USDT"] = &types.UserBalance{
		UserID:    userID,
		Asset:     "USDT",
		Available: 500,
		Locked:    500,
	}

	err := Unlock(s, userID, "USDT", 200)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	balance := us.Balances["USDT"]
	if balance.Available != 700 {
		t.Errorf("expected available 700, got %d", balance.Available)
	}
	if balance.Locked != 300 {
		t.Errorf("expected locked 300, got %d", balance.Locked)
	}
}

func TestUnlockInvalidAmount(t *testing.T) {
	s := state.New()
	userID := types.UserID(1)

	us := s.GetUserState(userID)
	us.Balances["USDT"] = &types.UserBalance{
		UserID:    userID,
		Asset:     "USDT",
		Available: 500,
		Locked:    200,
	}

	err := Unlock(s, userID, "USDT", 300)
	if err != ErrInvalidUnlockAmount {
		t.Errorf("expected ErrInvalidUnlockAmount, got %v", err)
	}
}

func TestTransferToMargin(t *testing.T) {
	s := state.New()
	userID := types.UserID(1)

	us := s.GetUserState(userID)
	us.Balances["USDT"] = &types.UserBalance{
		UserID:    userID,
		Asset:     "USDT",
		Available: 1000,
		Locked:    0,
		Margin:    0,
	}

	err := TransferToMargin(s, userID, "USDT", 500)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	balance := us.Balances["USDT"]
	if balance.Available != 500 {
		t.Errorf("expected available 500, got %d", balance.Available)
	}
	if balance.Margin != 500 {
		t.Errorf("expected margin 500, got %d", balance.Margin)
	}
}

func TestTransferFromMargin(t *testing.T) {
	s := state.New()
	userID := types.UserID(1)

	us := s.GetUserState(userID)
	us.Balances["USDT"] = &types.UserBalance{
		UserID:    userID,
		Asset:     "USDT",
		Available: 500,
		Locked:    0,
		Margin:    500,
	}

	err := TransferFromMargin(s, userID, "USDT", 300)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	balance := us.Balances["USDT"]
	if balance.Available != 800 {
		t.Errorf("expected available 800, got %d", balance.Available)
	}
	if balance.Margin != 200 {
		t.Errorf("expected margin 200, got %d", balance.Margin)
	}
}

func TestGetAvailable(t *testing.T) {
	s := state.New()
	userID := types.UserID(1)

	avail := GetAvailable(s, userID, "USDT")
	if avail != 0 {
		t.Errorf("expected 0, got %d", avail)
	}

	us := s.GetUserState(userID)
	us.Balances["USDT"] = &types.UserBalance{
		UserID:    userID,
		Asset:     "USDT",
		Available: 1000,
	}

	avail = GetAvailable(s, userID, "USDT")
	if avail != 1000 {
		t.Errorf("expected 1000, got %d", avail)
	}
}

func TestGetLocked(t *testing.T) {
	s := state.New()
	userID := types.UserID(1)

	us := s.GetUserState(userID)
	us.Balances["USDT"] = &types.UserBalance{
		UserID: userID,
		Asset:  "USDT",
		Locked: 500,
	}

	locked := GetLocked(s, userID, "USDT")
	if locked != 500 {
		t.Errorf("expected 500, got %d", locked)
	}
}

func TestGetMargin(t *testing.T) {
	s := state.New()
	userID := types.UserID(1)

	us := s.GetUserState(userID)
	us.Balances["USDT"] = &types.UserBalance{
		UserID: userID,
		Asset:  "USDT",
		Margin: 1000,
	}

	margin := GetMargin(s, userID, "USDT")
	if margin != 1000 {
		t.Errorf("expected 1000, got %d", margin)
	}
}
