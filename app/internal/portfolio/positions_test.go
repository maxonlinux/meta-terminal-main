package portfolio

import (
	"testing"

	"github.com/maxonlinux/meta-terminal-go/internal/registry"
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/robaho/fixed"
)

func TestUpdatePositionReduceRealizedPnL(t *testing.T) {
	reg := registry.New()
	reg.SetInstrument("BTCUSDT", &types.Instrument{
		Symbol:     "BTCUSDT",
		BaseAsset:  "BTC",
		QuoteAsset: "USDT",
	})
	svc, err := New(nil, reg)
	if err != nil {
		t.Fatalf("service: %v", err)
	}

	userID := types.UserID(1)
	quote := "USDT"
	svc.Balances[userID] = map[string]*types.Balance{
		quote: {UserID: userID, Asset: quote, Available: qty(0), Locked: qty(0), Margin: qty(0)},
	}

	pos := svc.GetPosition(userID, "BTCUSDT")
	pos.Size = qty(10)
	pos.EntryPrice = price(100)
	pos.Leverage = types.Leverage(fixed.NewI(2, 0))

	order := &types.Order{ID: 10, UserID: userID, Side: constants.ORDER_SIDE_SELL}
	match := &types.Match{
		Symbol:     "BTCUSDT",
		Category:   constants.CATEGORY_LINEAR,
		Price:      price(110),
		Quantity:   qty(4),
		TakerOrder: order,
		MakerOrder: order,
	}

	svc.updatePosition(userID, match, order)

	if math.Cmp(pos.Size, qty(6)) != 0 {
		t.Fatalf("expected size 6, got %s", pos.Size.String())
	}
	if math.Cmp(pos.EntryPrice, price(100)) != 0 {
		t.Fatalf("expected entry 100, got %s", pos.EntryPrice.String())
	}
	if math.Cmp(svc.GetBalance(userID, quote).Available, qty(40)) != 0 {
		t.Fatalf("expected realized PnL 40, got %s", svc.GetBalance(userID, quote).Available.String())
	}
}

func TestUpdatePositionFlipResetsEntry(t *testing.T) {
	reg := registry.New()
	reg.SetInstrument("BTCUSDT", &types.Instrument{
		Symbol:     "BTCUSDT",
		BaseAsset:  "BTC",
		QuoteAsset: "USDT",
	})
	svc, err := New(nil, reg)
	if err != nil {
		t.Fatalf("service: %v", err)
	}

	userID := types.UserID(1)
	quote := "USDT"
	svc.Balances[userID] = map[string]*types.Balance{
		quote: {UserID: userID, Asset: quote, Available: qty(0), Locked: qty(0), Margin: qty(0)},
	}

	pos := svc.GetPosition(userID, "BTCUSDT")
	pos.Size = qty(10)
	pos.EntryPrice = price(100)
	pos.Leverage = types.Leverage(fixed.NewI(2, 0))

	order := &types.Order{ID: 11, UserID: userID, Side: constants.ORDER_SIDE_SELL}
	match := &types.Match{
		Symbol:     "BTCUSDT",
		Category:   constants.CATEGORY_LINEAR,
		Price:      price(110),
		Quantity:   qty(15),
		TakerOrder: order,
		MakerOrder: order,
	}

	svc.updatePosition(userID, match, order)

	if math.Cmp(pos.Size, qty(-5)) != 0 {
		t.Fatalf("expected size -5, got %s", pos.Size.String())
	}
	if math.Cmp(pos.EntryPrice, price(110)) != 0 {
		t.Fatalf("expected entry 110 after flip, got %s", pos.EntryPrice.String())
	}
	if math.Cmp(svc.GetBalance(userID, quote).Available, qty(100)) != 0 {
		t.Fatalf("expected realized PnL 100, got %s", svc.GetBalance(userID, quote).Available.String())
	}
}
