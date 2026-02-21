package engine

import (
	"testing"

	"github.com/maxonlinux/meta-terminal-go/internal/registry"
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/robaho/fixed"
)

func TestReduceOnlyIntegration(t *testing.T) {
	reg := registry.New()
	reg.SetInstrument("BTCUSDT", &types.Instrument{
		Symbol:     "BTCUSDT",
		BaseAsset:  "BTC",
		QuoteAsset: "USDT",
		MinQty:     types.Quantity(fixed.NewI(1, 0)),
		TickSize:   types.Price(fixed.NewI(1, 0)),
		StepSize:   types.Quantity(fixed.NewI(1, 0)),
	})

	eng, err := NewEngine(nil, reg, nil)
	if err != nil {
		t.Fatalf("engine: %v", err)
	}

	order1 := eng.store.Build(
		types.UserID(1),
		"BTCUSDT",
		constants.CATEGORY_LINEAR,
		constants.ORDER_ORIGIN_USER,
		constants.ORDER_SIDE_SELL,
		constants.ORDER_TYPE_LIMIT,
		constants.TIF_GTC,
		types.Price(fixed.NewI(100, 0)),
		types.Quantity(fixed.NewI(6, 0)),
		types.Price{},
		true,
		false,
		0,
		constants.TRIGGER_DIRECTION_NONE,
	)
	order2 := eng.store.Build(
		types.UserID(1),
		"BTCUSDT",
		constants.CATEGORY_LINEAR,
		constants.ORDER_ORIGIN_USER,
		constants.ORDER_SIDE_SELL,
		constants.ORDER_TYPE_LIMIT,
		constants.TIF_GTC,
		types.Price(fixed.NewI(110, 0)),
		types.Quantity(fixed.NewI(4, 0)),
		types.Price{},
		true,
		false,
		0,
		constants.TRIGGER_DIRECTION_NONE,
	)

	eng.store.Add(order1)
	eng.store.Add(order2)

	eng.onPositionReduce(types.UserID(1), "BTCUSDT", types.Quantity(fixed.NewI(4, 0)))

	remaining := math.Zero
	for _, order := range []*types.Order{order1, order2} {
		if order.Status == constants.ORDER_STATUS_CANCELED || order.Status == constants.ORDER_STATUS_DEACTIVATED {
			continue
		}
		remaining = math.Add(remaining, math.Sub(order.Quantity, order.Filled))
	}

	if math.Cmp(remaining, types.Quantity(fixed.NewI(4, 0))) != 0 {
		t.Fatalf("expected remaining reduce-only qty 4, got %s", remaining.String())
	}
}
