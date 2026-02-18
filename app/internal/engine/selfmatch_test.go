package engine

import (
	"testing"

	"github.com/maxonlinux/meta-terminal-go/internal/registry"
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/robaho/fixed"
)

func TestSelfMatchRejected(t *testing.T) {
	reg := registry.New()
	reg.SetInstrument("BTCUSDT", &types.Instrument{
		Symbol:     "BTCUSDT",
		BaseAsset:  "BTC",
		QuoteAsset: "USDT",
		MinQty:     types.Quantity(fixed.NewI(0, 0)),
		TickSize:   types.Price(fixed.NewI(1, 0)),
		StepSize:   types.Quantity(fixed.NewI(1, 0)),
	})

	eng, err := NewEngine(nil, reg, nil)
	if err != nil {
		t.Fatalf("engine: %v", err)
	}
	eng.Portfolio().LoadBalance(&types.Balance{
		UserID:    1,
		Asset:     "USDT",
		Available: types.Quantity(fixed.NewI(1000, 0)),
	})

	price := types.Price(fixed.NewI(100, 0))
	qty := types.Quantity(fixed.NewI(1, 0))

	res := eng.Cmd(&PlaceOrderCmd{Req: &types.PlaceOrderRequest{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Price:    price,
		Quantity: qty,
	}})
	if res.Err != nil {
		t.Fatalf("maker order failed: %v", res.Err)
	}

	res = eng.Cmd(&PlaceOrderCmd{Req: &types.PlaceOrderRequest{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Price:    price,
		Quantity: qty,
	}})
	if res.Err != constants.ErrSelfMatch {
		t.Fatalf("expected self-match error, got %v", res.Err)
	}
}
