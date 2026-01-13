package core_test

import (
	"testing"

	"github.com/anomalyco/meta-terminal-go/core"
)

func TestBalanceService_Reserve(t *testing.T) {
	svc := core.NewBalanceService()
	var userID core.UserID = 1

	svc.Deposit(userID, "USDT", 10000)

	err := svc.Reserve(userID, "USDT", 500)
	if err != nil {
		t.Fatalf("Reserve failed: %v", err)
	}

	balance := svc.Get(userID, "USDT")
	if balance.Available != 9500 {
		t.Fatalf("Expected Available 9500, got %d", balance.Available)
	}
	if balance.Locked != 500 {
		t.Fatalf("Expected Locked 500, got %d", balance.Locked)
	}
}

func TestBalanceService_ReserveInsufficient(t *testing.T) {
	svc := core.NewBalanceService()
	var userID core.UserID = 1

	svc.Deposit(userID, "USDT", 100)

	err := svc.Reserve(userID, "USDT", 500)
	if err != core.ErrInsufficientBalance {
		t.Fatalf("Expected ErrInsufficientBalance, got %v", err)
	}

	balance := svc.Get(userID, "USDT")
	if balance.Available != 100 {
		t.Fatalf("Available should remain unchanged: %d", balance.Available)
	}
	if balance.Locked != 0 {
		t.Fatalf("Locked should remain 0: %d", balance.Locked)
	}
}

func TestBalanceService_Release(t *testing.T) {
	svc := core.NewBalanceService()
	var userID core.UserID = 1

	svc.Deposit(userID, "USDT", 10000)
	svc.Reserve(userID, "USDT", 500)
	svc.Release(userID, "USDT", 200)

	balance := svc.Get(userID, "USDT")
	if balance.Available != 9700 {
		t.Fatalf("Expected Available 9700, got %d", balance.Available)
	}
	if balance.Locked != 300 {
		t.Fatalf("Expected Locked 300, got %d", balance.Locked)
	}
}

func TestBalanceService_Deduct(t *testing.T) {
	svc := core.NewBalanceService()
	var userID core.UserID = 1

	svc.Deposit(userID, "USDT", 10000)
	svc.Reserve(userID, "USDT", 500)
	svc.Deduct(userID, "USDT", 300)

	balance := svc.Get(userID, "USDT")
	if balance.Available != 9500 {
		t.Fatalf("Expected Available 9500, got %d", balance.Available)
	}
	if balance.Locked != 200 {
		t.Fatalf("Expected Locked 200, got %d", balance.Locked)
	}
}

func TestBalanceService_AddCredit(t *testing.T) {
	svc := core.NewBalanceService()
	var userID core.UserID = 1

	svc.Deposit(userID, "USDT", 5000)
	svc.AddCredit(userID, "USDT", 1000)

	balance := svc.Get(userID, "USDT")
	if balance.Available != 6000 {
		t.Fatalf("Expected Available 6000, got %d", balance.Available)
	}
}

func TestBalanceService_Margin(t *testing.T) {
	svc := core.NewBalanceService()
	var userID core.UserID = 1

	svc.Deposit(userID, "USDT", 10000)
	svc.AddMargin(userID, "USDT", 500)

	balance := svc.Get(userID, "USDT")
	if balance.Margin != 500 {
		t.Fatalf("Expected Margin 500, got %d", balance.Margin)
	}
}

func TestBalanceService_UseMargin(t *testing.T) {
	svc := core.NewBalanceService()
	var userID core.UserID = 1

	svc.AddMargin(userID, "USDT", 1000)
	err := svc.UseMargin(userID, "USDT", 500)
	if err != nil {
		t.Fatalf("UseMargin failed: %v", err)
	}

	balance := svc.Get(userID, "USDT")
	if balance.Margin != 500 {
		t.Fatalf("Expected Margin 500, got %d", balance.Margin)
	}
	if balance.Locked != 500 {
		t.Fatalf("Expected Locked 500, got %d", balance.Locked)
	}
}

func TestBalanceService_UseMarginInsufficient(t *testing.T) {
	svc := core.NewBalanceService()
	var userID core.UserID = 1

	svc.AddMargin(userID, "USDT", 100)
	err := svc.UseMargin(userID, "USDT", 500)
	if err != core.ErrInsufficientBalance {
		t.Fatalf("Expected ErrInsufficientBalance, got %v", err)
	}
}

func TestBalanceService_TransferMargin(t *testing.T) {
	svc := core.NewBalanceService()
	var userID core.UserID = 1

	svc.AddMargin(userID, "BTCUSDT", 1000)
	err := svc.TransferMargin(userID, "BTCUSDT", "ETHUSDT", 500)
	if err != nil {
		t.Fatalf("TransferMargin failed: %v", err)
	}

	btcBalance := svc.Get(userID, "BTCUSDT")
	ethBalance := svc.Get(userID, "ETHUSDT")

	if btcBalance.Margin != 500 {
		t.Fatalf("Expected BTCUSDT Margin 500, got %d", btcBalance.Margin)
	}
	if ethBalance.Margin != 500 {
		t.Fatalf("Expected ETHUSDT Margin 500, got %d", ethBalance.Margin)
	}
}

func TestBalanceService_DepositWithdraw(t *testing.T) {
	svc := core.NewBalanceService()
	var userID core.UserID = 1

	svc.Deposit(userID, "USDT", 5000)

	balance := svc.Get(userID, "USDT")
	if balance.Available != 5000 {
		t.Fatalf("Expected Available 5000, got %d", balance.Available)
	}

	err := svc.Withdraw(userID, "USDT", 2000)
	if err != nil {
		t.Fatalf("Withdraw failed: %v", err)
	}

	balance = svc.Get(userID, "USDT")
	if balance.Available != 3000 {
		t.Fatalf("Expected Available 3000, got %d", balance.Available)
	}
}

func TestBalanceService_WithdrawInsufficient(t *testing.T) {
	svc := core.NewBalanceService()
	var userID core.UserID = 1

	svc.Deposit(userID, "USDT", 500)
	err := svc.Withdraw(userID, "USDT", 1000)
	if err != core.ErrInsufficientBalance {
		t.Fatalf("Expected ErrInsufficientBalance, got %v", err)
	}
}

func TestBalanceService_GetAvailable(t *testing.T) {
	svc := core.NewBalanceService()
	var userID core.UserID = 1

	svc.Deposit(userID, "USDT", 5000)
	svc.AddMargin(userID, "USDT", 2000)
	svc.Reserve(userID, "USDT", 1000)

	available := svc.GetAvailable(userID, "USDT")
	if available != 6000 {
		t.Fatalf("Expected Available 6000, got %d", available)
	}
}

func TestBalanceService_GetTotal(t *testing.T) {
	svc := core.NewBalanceService()
	var userID core.UserID = 1

	svc.Deposit(userID, "USDT", 5000)
	svc.AddMargin(userID, "USDT", 2000)
	svc.Reserve(userID, "USDT", 1000)

	total := svc.GetTotal(userID, "USDT")
	if total != 7000 {
		t.Fatalf("Expected Total 7000, got %d", total)
	}
}

func TestBalanceService_MultipleUsers(t *testing.T) {
	svc := core.NewBalanceService()

	svc.Deposit(core.UserID(1), "USDT", 10000)
	svc.Deposit(core.UserID(2), "USDT", 20000)

	balance1 := svc.Get(core.UserID(1), "USDT")
	balance2 := svc.Get(core.UserID(2), "USDT")

	if balance1.Available != 10000 {
		t.Fatalf("User1: Expected Available 10000, got %d", balance1.Available)
	}
	if balance2.Available != 20000 {
		t.Fatalf("User2: Expected Available 20000, got %d", balance2.Available)
	}
}

func TestBalanceService_MultipleSymbols(t *testing.T) {
	svc := core.NewBalanceService()
	var userID core.UserID = 1

	svc.Deposit(userID, "BTCUSDT", 1000)
	svc.Deposit(userID, "ETHUSDT", 5000)

	btcBalance := svc.Get(userID, "BTCUSDT")
	ethBalance := svc.Get(userID, "ETHUSDT")

	if btcBalance.Available != 1000 {
		t.Fatalf("BTCUSDT: Expected 1000, got %d", btcBalance.Available)
	}
	if ethBalance.Available != 5000 {
		t.Fatalf("ETHUSDT: Expected 5000, got %d", ethBalance.Available)
	}
}

func TestBalanceService_Count(t *testing.T) {
	svc := core.NewBalanceService()

	svc.Deposit(core.UserID(1), "BTCUSDT", 1000)
	svc.Deposit(core.UserID(1), "ETHUSDT", 2000)
	svc.Deposit(core.UserID(2), "BTCUSDT", 3000)

	count := svc.Count()
	if count != 3 {
		t.Fatalf("Expected count 3, got %d", count)
	}
}

func TestBalanceService_GetNonExistent(t *testing.T) {
	svc := core.NewBalanceService()

	balance := svc.Get(core.UserID(999), "BTCUSDT")
	if balance != nil {
		t.Fatalf("Expected nil balance for non-existent user, got %v", balance)
	}
}
