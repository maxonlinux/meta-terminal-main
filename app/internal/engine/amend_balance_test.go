package engine

import (
	"testing"

	"github.com/maxonlinux/meta-terminal-go/internal/registry"
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/robaho/fixed"
)

func TestAmendBalanceSpotBuy(t *testing.T) {
	eng, _ := newEngineWithInstrument(t, "BTCUSDT")
	loadBalance(eng, 1, "USDT", qty(1000000))

	place := &PlaceOrderCmd{Req: &types.PlaceOrderRequest{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_SPOT,
		Origin:   constants.ORDER_ORIGIN_SYSTEM,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Price:    price(100),
		Quantity: qty(10),
	}}
	res := eng.Cmd(place)
	if res.Err != nil {
		t.Fatalf("place: %v", res.Err)
	}

	bal := eng.Portfolio().GetBalance(1, "USDT")
	assertQty(t, "avail after place", bal.Available, qty(999000))
	assertQty(t, "locked after place", bal.Locked, qty(1000))

	res = eng.Cmd(&AmendOrderCmd{UserID: 1, OrderID: res.Order.ID, NewQty: qty(12), NewPrice: price(110)})
	if res.Err != nil {
		t.Fatalf("amend up: %v", res.Err)
	}
	bal = eng.Portfolio().GetBalance(1, "USDT")
	assertQty(t, "avail after amend up", bal.Available, qty(998680))
	assertQty(t, "locked after amend up", bal.Locked, qty(1320))

	res = eng.Cmd(&AmendOrderCmd{UserID: 1, OrderID: res.Order.ID, NewQty: qty(8), NewPrice: price(90)})
	if res.Err != nil {
		t.Fatalf("amend down: %v", res.Err)
	}
	bal = eng.Portfolio().GetBalance(1, "USDT")
	assertQty(t, "avail after amend down", bal.Available, qty(999280))
	assertQty(t, "locked after amend down", bal.Locked, qty(720))
}

func TestAmendBalanceSpotSell(t *testing.T) {
	eng, _ := newEngineWithInstrument(t, "BTCUSDT")
	loadBalance(eng, 2, "BTC", qty(1000))

	place := &PlaceOrderCmd{Req: &types.PlaceOrderRequest{
		UserID:   2,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_SPOT,
		Origin:   constants.ORDER_ORIGIN_SYSTEM,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Price:    price(100),
		Quantity: qty(10),
	}}
	res := eng.Cmd(place)
	if res.Err != nil {
		t.Fatalf("place: %v", res.Err)
	}

	bal := eng.Portfolio().GetBalance(2, "BTC")
	assertQty(t, "avail after place", bal.Available, qty(990))
	assertQty(t, "locked after place", bal.Locked, qty(10))

	res = eng.Cmd(&AmendOrderCmd{UserID: 2, OrderID: res.Order.ID, NewQty: qty(12), NewPrice: price(90)})
	if res.Err != nil {
		t.Fatalf("amend up: %v", res.Err)
	}
	bal = eng.Portfolio().GetBalance(2, "BTC")
	assertQty(t, "avail after amend up", bal.Available, qty(988))
	assertQty(t, "locked after amend up", bal.Locked, qty(12))

	res = eng.Cmd(&AmendOrderCmd{UserID: 2, OrderID: res.Order.ID, NewQty: qty(5), NewPrice: price(120)})
	if res.Err != nil {
		t.Fatalf("amend down: %v", res.Err)
	}
	bal = eng.Portfolio().GetBalance(2, "BTC")
	assertQty(t, "avail after amend down", bal.Available, qty(995))
	assertQty(t, "locked after amend down", bal.Locked, qty(5))
}

func TestAmendBalanceLinear(t *testing.T) {
	eng, _ := newEngineWithInstrument(t, "BTCUSDT")
	loadBalance(eng, 3, "USDT", qty(1000000))

	place := &PlaceOrderCmd{Req: &types.PlaceOrderRequest{
		UserID:   3,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Origin:   constants.ORDER_ORIGIN_SYSTEM,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Price:    price(100),
		Quantity: qty(10),
	}}
	res := eng.Cmd(place)
	if res.Err != nil {
		t.Fatalf("place: %v", res.Err)
	}

	bal := eng.Portfolio().GetBalance(3, "USDT")
	assertQty(t, "avail after place", bal.Available, qty(999500))
	assertQty(t, "locked after place", bal.Locked, qty(500))

	res = eng.Cmd(&AmendOrderCmd{UserID: 3, OrderID: res.Order.ID, NewQty: qty(12), NewPrice: price(110)})
	if res.Err != nil {
		t.Fatalf("amend up: %v", res.Err)
	}
	bal = eng.Portfolio().GetBalance(3, "USDT")
	assertQty(t, "avail after amend up", bal.Available, qty(999340))
	assertQty(t, "locked after amend up", bal.Locked, qty(660))

	res = eng.Cmd(&AmendOrderCmd{UserID: 3, OrderID: res.Order.ID, NewQty: qty(8), NewPrice: price(90)})
	if res.Err != nil {
		t.Fatalf("amend down: %v", res.Err)
	}
	bal = eng.Portfolio().GetBalance(3, "USDT")
	assertQty(t, "avail after amend down", bal.Available, qty(999640))
	assertQty(t, "locked after amend down", bal.Locked, qty(360))
}

func TestAmendBalanceInsufficientReserve(t *testing.T) {
	eng, _ := newEngineWithInstrument(t, "BTCUSDT")
	loadBalance(eng, 4, "USDT", qty(600))

	place := &PlaceOrderCmd{Req: &types.PlaceOrderRequest{
		UserID:   4,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_SPOT,
		Origin:   constants.ORDER_ORIGIN_SYSTEM,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Price:    price(100),
		Quantity: qty(5),
	}}
	res := eng.Cmd(place)
	if res.Err != nil {
		t.Fatalf("place: %v", res.Err)
	}
	orderID := res.Order.ID

	bal := eng.Portfolio().GetBalance(4, "USDT")
	assertQty(t, "avail after place", bal.Available, qty(100))
	assertQty(t, "locked after place", bal.Locked, qty(500))

	res = eng.Cmd(&AmendOrderCmd{UserID: 4, OrderID: orderID, NewQty: qty(10), NewPrice: price(200)})
	if res.Err == nil {
		t.Fatalf("expected amend error")
	}

	order, ok := eng.Store().GetUserOrder(4, orderID)
	if !ok {
		t.Fatalf("order missing after failed amend")
	}
	if order.Price.Cmp(price(100)) != 0 || order.Quantity.Cmp(qty(5)) != 0 {
		t.Fatalf("order changed after failed amend")
	}

	bal = eng.Portfolio().GetBalance(4, "USDT")
	assertQty(t, "avail after failed amend", bal.Available, qty(100))
	assertQty(t, "locked after failed amend", bal.Locked, qty(500))
}

func TestAmendBalancePartialFillRelease(t *testing.T) {
	eng, _ := newEngineWithInstrument(t, "BTCUSDT")
	loadBalance(eng, 5, "USDT", qty(1000000))

	place := &PlaceOrderCmd{Req: &types.PlaceOrderRequest{
		UserID:   5,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_SPOT,
		Origin:   constants.ORDER_ORIGIN_SYSTEM,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Price:    price(100),
		Quantity: qty(10),
	}}
	res := eng.Cmd(place)
	if res.Err != nil {
		t.Fatalf("place: %v", res.Err)
	}
	orderID := res.Order.ID

	if err := eng.Store().Fill(5, orderID, qty(4)); err != nil {
		t.Fatalf("fill: %v", err)
	}
	eng.Portfolio().LoadBalance(&types.Balance{
		UserID:    5,
		Asset:     "USDT",
		Available: qty(999000),
		Locked:    qty(600),
		Margin:    qty(0),
	})

	res = eng.Cmd(&AmendOrderCmd{UserID: 5, OrderID: orderID, NewQty: qty(8), NewPrice: price(110)})
	if res.Err != nil {
		t.Fatalf("amend: %v", res.Err)
	}
	bal := eng.Portfolio().GetBalance(5, "USDT")
	assertQty(t, "avail after amend", bal.Available, qty(999160))
	assertQty(t, "locked after amend", bal.Locked, qty(440))
}

func newEngineWithInstrument(t *testing.T, symbol string) (*Engine, *registry.Registry) {
	t.Helper()
	reg := registry.New()
	reg.SetInstrument(symbol, &types.Instrument{
		Symbol:     symbol,
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
	return eng, reg
}

func loadBalance(eng *Engine, userID types.UserID, asset string, amount types.Quantity) {
	eng.Portfolio().LoadBalance(&types.Balance{UserID: userID, Asset: asset, Available: amount})
}

func price(v int64) types.Price {
	return types.Price(fixed.NewI(v, 0))
}

func qty(v int64) types.Quantity {
	return types.Quantity(fixed.NewI(v, 0))
}

func assertQty(t *testing.T, label string, got types.Quantity, want types.Quantity) {
	t.Helper()
	if math.Cmp(got, want) != 0 {
		t.Fatalf("%s: expected %s, got %s", label, want.String(), got.String())
	}
}
