package portfolio

import (
	"testing"

	"github.com/maxonlinux/meta-terminal-go/internal/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

func TestReserve(t *testing.T) {
	s := New(nil)
	s.GetBalance(1, "USDT").Available = 100

	if err := s.Reserve(1, "USDT", 50); err != nil {
		t.Fatal(err)
	}
	b := s.GetBalance(1, "USDT")
	if b.Available != 50 || b.Locked != 50 {
		t.Fatal(b)
	}
}

func TestRelease(t *testing.T) {
	s := New(nil)
	b := s.GetBalance(1, "USDT")
	b.Available, b.Locked = 50, 50

	s.Release(1, "USDT", 30)
	if b.Available != 80 || b.Locked != 20 {
		t.Fatal(b)
	}
}

func TestSpotTrade(t *testing.T) {
	s := New(nil)
	s.GetBalance(1, "USDT").Available = 5000000
	s.GetBalance(2, "BTC").Available = 100

	trade := &types.Trade{
		Symbol:     "BTCUSDT",
		Category:   constants.CATEGORY_SPOT,
		Quantity:   100,
		Price:      50000,
		TakerOrder: &types.Order{UserID: 1, Side: constants.ORDER_SIDE_BUY},
		MakerOrder: &types.Order{UserID: 2, Side: constants.ORDER_SIDE_SELL},
	}
	s.ExecuteTrade(trade, trade.TakerOrder, trade.MakerOrder)

	if s.GetBalance(1, "USDT").Available != 0 {
		t.Fatal("taker USDT")
	}
	if s.GetBalance(1, "BTC").Available != 100 {
		t.Fatal("taker BTC")
	}
	if s.GetBalance(2, "USDT").Available != 5000000 {
		t.Fatal("maker USDT")
	}
	if s.GetBalance(2, "BTC").Available != 0 {
		t.Fatal("maker BTC")
	}
}

func TestLinearPosition(t *testing.T) {
	s := New(nil)

	trade := &types.Trade{
		Symbol:     "BTCUSDT",
		Category:   constants.CATEGORY_LINEAR,
		Quantity:   100,
		Price:      50000,
		TakerOrder: &types.Order{UserID: 1, Side: constants.ORDER_SIDE_BUY},
	}
	s.ExecuteTrade(trade, trade.TakerOrder, nil)

	p := s.GetPosition(1, "BTCUSDT")
	if p.Size != 100 {
		t.Fatal(p)
	}
}

func TestPositionFlip(t *testing.T) {
	s := New(nil)
	p := s.GetPosition(1, "BTCUSDT")
	p.Size = 100

	trade := &types.Trade{
		Symbol:     "BTCUSDT",
		Category:   constants.CATEGORY_LINEAR,
		Quantity:   150,
		Price:      50000,
		TakerOrder: &types.Order{UserID: 1, Side: constants.ORDER_SIDE_SELL},
	}
	s.ExecuteTrade(trade, trade.TakerOrder, nil)

	if p.Size != -50 {
		t.Fatal(p)
	}
}
