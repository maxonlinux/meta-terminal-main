package portfolio

import (
	"testing"

	"github.com/maxonlinux/meta-terminal-go/internal/registry"
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/robaho/fixed"
)

func qty(v int64) types.Quantity {
	return types.Quantity(fixed.NewI(v, 0))
}

func price(v int64) types.Price {
	return types.Price(fixed.NewI(v, 0))
}

func TestExecuteTradeSpotBalances(t *testing.T) {
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

	base := "BTC"
	quote := "USDT"

	svc.Balances[types.UserID(1)] = map[string]*types.Balance{
		quote: {UserID: 1, Asset: quote, Available: qty(50000), Locked: qty(50000)},
		base:  {UserID: 1, Asset: base, Available: qty(0)},
	}
	svc.Balances[types.UserID(2)] = map[string]*types.Balance{
		quote: {UserID: 2, Asset: quote, Available: qty(0)},
		base:  {UserID: 2, Asset: base, Available: qty(0), Locked: qty(1)},
	}

	taker := &types.Order{UserID: 1, Side: constants.ORDER_SIDE_BUY}
	maker := &types.Order{UserID: 2, Side: constants.ORDER_SIDE_SELL}
	match := &types.Match{
		Symbol:     "BTCUSDT",
		Category:   constants.CATEGORY_SPOT,
		Price:      price(50000),
		Quantity:   qty(1),
		TakerOrder: taker,
		MakerOrder: maker,
	}

	if err := svc.ExecuteTrade(match); err != nil {
		t.Fatalf("execute trade: %v", err)
	}

	if math.Sign(svc.GetBalance(1, base).Available) == 0 {
		t.Fatalf("expected taker base balance increase")
	}
	if svc.GetBalance(1, quote).Available.Cmp(qty(50000)) != 0 {
		t.Fatalf("expected taker quote decrease to 50000")
	}
	if svc.GetBalance(2, quote).Available.Cmp(qty(50000)) != 0 {
		t.Fatalf("expected maker quote increase to 50000")
	}
	if svc.GetBalance(2, base).Available.Cmp(qty(0)) != 0 {
		t.Fatalf("expected maker base decrease to 0")
	}
}

func TestExecuteTradeLinearPosition(t *testing.T) {
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
	quote := "USDT"

	svc.Balances[types.UserID(1)] = map[string]*types.Balance{
		quote: {UserID: 1, Asset: quote, Available: qty(100000), Locked: qty(1000)},
	}
	svc.Balances[types.UserID(2)] = map[string]*types.Balance{
		quote: {UserID: 2, Asset: quote, Available: qty(100000), Locked: qty(1000)},
	}

	taker := &types.Order{UserID: 1, Side: constants.ORDER_SIDE_BUY}
	maker := &types.Order{UserID: 2, Side: constants.ORDER_SIDE_SELL}
	match := &types.Match{
		Symbol:     "BTCUSDT",
		Category:   constants.CATEGORY_LINEAR,
		Price:      price(100),
		Quantity:   qty(10),
		TakerOrder: taker,
		MakerOrder: maker,
	}

	if err := svc.ExecuteTrade(match); err != nil {
		t.Fatalf("execute trade: %v", err)
	}

	if pos := svc.GetPosition(1, "BTCUSDT"); math.Sign(pos.Size) <= 0 {
		t.Fatalf("expected taker long position")
	}
	if pos := svc.GetPosition(2, "BTCUSDT"); math.Sign(pos.Size) >= 0 {
		t.Fatalf("expected maker short position")
	}
	if math.Sign(svc.GetBalance(1, quote).Margin) == 0 {
		t.Fatalf("expected taker margin increase")
	}
	if math.Sign(svc.GetBalance(2, quote).Margin) == 0 {
		t.Fatalf("expected maker margin increase")
	}
}
