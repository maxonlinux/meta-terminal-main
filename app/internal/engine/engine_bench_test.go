package engine

import (
	"testing"

	"github.com/maxonlinux/meta-terminal-go/internal/registry"
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/outbox"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/robaho/fixed"
)

func BenchmarkEnginePipeline(b *testing.B) {
	root := b.TempDir()
	ob, err := outbox.OpenWithOptions(root, outbox.Options{QueueSize: 1 << 16, SegmentSize: 1 << 20})
	if err != nil {
		b.Fatal(err)
	}
	ob.Start()
	b.Cleanup(func() {
		_ = ob.Close()
	})

	reg := registry.New()
	reg.SetInstrument("BTCUSDT", &types.Instrument{
		Symbol:     "BTCUSDT",
		BaseAsset:  "BTC",
		QuoteAsset: "USDT",
		MinQty:     types.Quantity(fixed.NewI(1, 0)),
		TickSize:   types.Price(fixed.NewI(1, 0)),
		StepSize:   types.Quantity(fixed.NewI(1, 0)),
	})

	eng, err := NewEngine(ob, reg, nil)
	if err != nil {
		b.Fatalf("engine: %v", err)
	}

	price := types.Price(fixed.NewI(1, 0))
	qty := types.Quantity(fixed.NewI(1, 0))
	bulkQty := types.Quantity(fixed.NewI(100000000, 0))
	book, err := eng.getBook(constants.CATEGORY_SPOT, "BTCUSDT")
	if err != nil {
		b.Fatal(err)
	}
	makerOrder := eng.store.Build(
		types.UserID(2),
		"BTCUSDT",
		constants.CATEGORY_SPOT,
		constants.ORDER_ORIGIN_USER,
		constants.ORDER_SIDE_SELL,
		constants.ORDER_TYPE_LIMIT,
		constants.TIF_GTC,
		price,
		bulkQty,
		types.Price{},
		false,
		false,
		0,
	)
	eng.store.Add(makerOrder)
	book.Add(makerOrder)

	eng.portfolio.Balances[types.UserID(1)] = map[string]*types.Balance{
		"USDT": {UserID: 1, Asset: "USDT", Available: types.Quantity(fixed.NewI(1000000000, 0))},
	}
	eng.portfolio.Balances[types.UserID(2)] = map[string]*types.Balance{
		"BTC": {UserID: 2, Asset: "BTC", Available: types.Quantity(fixed.NewI(0, 0)), Locked: types.Quantity(fixed.NewI(1000000000, 0))},
	}

	b.ReportAllocs()
	b.ResetTimer()
	takerReq := &types.PlaceOrderRequest{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_SPOT,
		Origin:   constants.ORDER_ORIGIN_USER,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_IOC,
		Price:    price,
		Quantity: qty,
	}
	cmd := &PlaceOrderCmd{Req: takerReq}

	for i := 0; i < b.N; i++ {
		if res := eng.Cmd(cmd); res.Err != nil {
			b.Fatalf("benchmark failed: %v", res.Err)
		}
	}
}
