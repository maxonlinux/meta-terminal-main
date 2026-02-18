package mm

import (
	"math/rand"
	"testing"

	"github.com/maxonlinux/meta-terminal-go/internal/engine"
	"github.com/maxonlinux/meta-terminal-go/internal/registry"
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/robaho/fixed"
)

func TestMarketMakerAmendKeepsOrderIDs(t *testing.T) {
	reg := registry.New()
	reg.SetInstrument("BTCUSDT", &types.Instrument{
		Symbol:     "BTCUSDT",
		BaseAsset:  "BTC",
		QuoteAsset: "USDT",
		MinQty:     types.Quantity(fixed.NewI(1, 0)),
		TickSize:   types.Price(fixed.NewI(1, 0)),
		StepSize:   types.Quantity(fixed.NewI(1, 0)),
	})

	eng, err := engine.NewEngine(nil, reg, nil)
	if err != nil {
		t.Fatalf("engine: %v", err)
	}

	mm := New(eng, reg, Config{
		Levels:        2,
		CancelPercent: 0,
		SkipPercent:   0,
		BotUserID:     777,
	})
	mm.val = rand.New(rand.NewSource(1))

	reg.SetPrice("BTCUSDT", registry.PriceTick{Price: types.Price(fixed.NewI(100, 0))})
	mm.refresh()

	key := marketKey{symbol: "BTCUSDT", category: constants.CATEGORY_SPOT}
	orders := mm.orders[key]
	if len(orders) != 4 {
		t.Fatalf("expected 4 spot orders, got %d", len(orders))
	}
	firstIDs := map[string]types.OrderID{}
	for level, id := range orders {
		firstIDs[level] = id
	}

	reg.SetPrice("BTCUSDT", registry.PriceTick{Price: types.Price(fixed.NewI(105, 0))})
	mm.refresh()

	orders = mm.orders[key]
	if len(orders) != 4 {
		t.Fatalf("expected 4 spot orders after refresh, got %d", len(orders))
	}
	for level, id := range orders {
		if prev, ok := firstIDs[level]; !ok || prev != id {
			t.Fatalf("order id changed for level %s", level)
		}
	}
}

func TestMarketMakerPlacesAllMarkets(t *testing.T) {
	reg := registry.New()
	reg.SetInstrument("BTCUSDT", &types.Instrument{
		Symbol:     "BTCUSDT",
		BaseAsset:  "BTC",
		QuoteAsset: "USDT",
		MinQty:     types.Quantity(fixed.NewI(1, 0)),
		TickSize:   types.Price(fixed.NewI(1, 0)),
		StepSize:   types.Quantity(fixed.NewI(1, 0)),
	})
	reg.SetInstrument("ETHUSDT", &types.Instrument{
		Symbol:     "ETHUSDT",
		BaseAsset:  "ETH",
		QuoteAsset: "USDT",
		MinQty:     types.Quantity(fixed.NewI(1, 0)),
		TickSize:   types.Price(fixed.NewI(1, 0)),
		StepSize:   types.Quantity(fixed.NewI(1, 0)),
	})

	eng, err := engine.NewEngine(nil, reg, nil)
	if err != nil {
		t.Fatalf("engine: %v", err)
	}

	mm := New(eng, reg, Config{
		Levels:        1,
		CancelPercent: 0,
		SkipPercent:   0,
		BotUserID:     777,
		MinBalance:    500000,
	})
	mm.val = rand.New(rand.NewSource(1))
	eng.Portfolio().LoadBalance(&types.Balance{UserID: 777, Asset: "USDT", Available: types.Quantity(fixed.NewI(1, 0))})
	eng.Portfolio().LoadBalance(&types.Balance{UserID: 777, Asset: "BTC", Available: types.Quantity(fixed.NewI(1, 0))})
	eng.Portfolio().LoadBalance(&types.Balance{UserID: 777, Asset: "ETH", Available: types.Quantity(fixed.NewI(1, 0))})

	reg.SetPrice("BTCUSDT", registry.PriceTick{Price: types.Price(fixed.NewI(100, 0))})
	reg.SetPrice("ETHUSDT", registry.PriceTick{Price: types.Price(fixed.NewI(200, 0))})
	mm.refresh()

	keys := []marketKey{
		{symbol: "BTCUSDT", category: constants.CATEGORY_SPOT},
		{symbol: "BTCUSDT", category: constants.CATEGORY_LINEAR},
		{symbol: "ETHUSDT", category: constants.CATEGORY_SPOT},
		{symbol: "ETHUSDT", category: constants.CATEGORY_LINEAR},
	}
	for _, key := range keys {
		orders := mm.orders[key]
		if len(orders) != 2 {
			t.Fatalf("expected 2 orders for %s/%d, got %d", key.symbol, key.category, len(orders))
		}
	}

	bot := types.UserID(777)
	for _, asset := range []string{"BTC", "ETH", "USDT"} {
		bal := eng.Portfolio().GetBalance(bot, asset)
		if bal == nil {
			t.Fatalf("expected balance for %s", asset)
		}
		if bal.Available.Cmp(types.Quantity(fixed.NewI(500000, 0))) < 0 {
			t.Fatalf("expected min balance for %s, got %s", asset, bal.Available.String())
		}
	}
}
