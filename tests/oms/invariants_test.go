package oms_test

import (
	"context"
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func TestBalancesRemainNonNegativeAfterTrades(t *testing.T) {
	s, port := newService()

	makerID := types.UserID(1)
	takerID := types.UserID(2)

	setBalance(port, makerID, "BTC", 5, 0, 0)
	setBalance(port, makerID, "USDT", 0, 0, 0)
	setBalance(port, takerID, "BTC", 0, 0, 0)
	setBalance(port, takerID, "USDT", 100000, 0, 0)

	_, err := s.PlaceOrder(context.Background(), &types.OrderInput{
		UserID:   makerID,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_SPOT,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 2,
		Price:    1000,
	})
	if err != nil {
		t.Fatalf("maker place: %v", err)
	}

	_, err = s.PlaceOrder(context.Background(), &types.OrderInput{
		UserID:   takerID,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_SPOT,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_IOC,
		Quantity: 2,
		Price:    1000,
	})
	if err != nil {
		t.Fatalf("taker place: %v", err)
	}

	for userID, balances := range port.Balances {
		for asset, bal := range balances {
			if bal.Available < 0 || bal.Locked < 0 || bal.Margin < 0 {
				t.Fatalf("negative balance: user=%d asset=%s %+v", userID, asset, bal)
			}
		}
	}
}
