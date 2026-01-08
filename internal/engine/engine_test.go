package engine

import (
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/history"
	"github.com/anomalyco/meta-terminal-go/internal/idgen"
	"github.com/anomalyco/meta-terminal-go/internal/linear"
	"github.com/anomalyco/meta-terminal-go/internal/market"
	"github.com/anomalyco/meta-terminal-go/internal/orderbook"
	"github.com/anomalyco/meta-terminal-go/internal/orderstore"
	"github.com/anomalyco/meta-terminal-go/internal/registry"
	"github.com/anomalyco/meta-terminal-go/internal/spot"
	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func newTestEngine() *Engine {
	orders := orderstore.New()
	idGen := idgen.NewSnowflake(0)
	books := orderbook.NewStateWithIDGenerator(idGen)
	users := state.NewUsers()
	reg := registry.New()
	triggers := state.NewTriggers()
	var hist history.Reader

	spotClearing := spot.NewClearing(users, reg)
	spotMarket := spot.NewMarket(books, spotClearing)

	linearValidator := linear.NewValidator(users)
	linearClearing := linear.NewClearing(users, reg)
	linearMarket := linear.NewMarket(books, linearValidator, linearClearing)

	markets := map[int8]market.Market{
		spotMarket.GetCategory():   spotMarket,
		linearMarket.GetCategory(): linearMarket,
	}

	eng := New(orders, books, users, reg, triggers, hist, nil, markets, idGen)
	eng.AddInstrument("BTCUSDT", constants.CATEGORY_SPOT, 50000)
	eng.AddInstrument("BTCUSDT", constants.CATEGORY_LINEAR, 50000)
	return eng
}

func TestPlaceOrderAndMatch(t *testing.T) {
	eng := newTestEngine()
	_ = eng.SetBalance(1, "USDT", 1_000_000_000)
	_ = eng.SetBalance(2, "BTC", 1_000_000)

	buy := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_SPOT,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 100,
		Price:    50000,
	}
	res, err := eng.PlaceOrder(buy)
	if err != nil {
		t.Fatalf("place buy failed: %v", err)
	}
	if res.Order.Status != constants.ORDER_STATUS_NEW {
		t.Fatalf("expected NEW, got %d", res.Order.Status)
	}

	sell := &types.OrderInput{
		UserID:   2,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_SPOT,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_IOC,
		Quantity: 50,
		Price:    50000,
	}
	resSell, err := eng.PlaceOrder(sell)
	if err != nil {
		t.Fatalf("place sell failed: %v", err)
	}
	if resSell.Order.Status != constants.ORDER_STATUS_FILLED {
		t.Fatalf("expected FILLED, got %d", resSell.Order.Status)
	}
	if len(resSell.Trades) == 0 {
		t.Fatalf("expected trade")
	}
}

func TestCancelOrder(t *testing.T) {
	eng := newTestEngine()
	_ = eng.SetBalance(1, "USDT", 1_000_000_000)

	buy := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_SPOT,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 100,
		Price:    50000,
	}
	res, err := eng.PlaceOrder(buy)
	if err != nil {
		t.Fatalf("place buy failed: %v", err)
	}
	if err := eng.CancelOrder(res.Order.ID, res.Order.UserID); err != nil {
		t.Fatalf("cancel failed: %v", err)
	}
	open := eng.OpenOrders(res.Order.UserID)
	if len(open) != 0 {
		t.Fatalf("expected no open orders")
	}
}

func TestConditionalTrigger(t *testing.T) {
	eng := newTestEngine()
	_ = eng.SetBalance(1, "USDT", 1_000_000_000)
	_ = eng.SetBalance(2, "BTC", 1_000_000)

	conditional := &types.OrderInput{
		UserID:       1,
		Symbol:       "BTCUSDT",
		Category:     constants.CATEGORY_LINEAR,
		Side:         constants.ORDER_SIDE_BUY,
		Type:         constants.ORDER_TYPE_LIMIT,
		TIF:          constants.TIF_GTC,
		Quantity:     10,
		Price:        50000,
		TriggerPrice: 48000,
		Leverage:     5,
	}
	res, err := eng.PlaceOrder(conditional)
	if err != nil {
		t.Fatalf("place conditional failed: %v", err)
	}
	if res.Order.Status != constants.ORDER_STATUS_UNTRIGGERED {
		t.Fatalf("expected UNTRIGGERED")
	}

	eng.OnPriceTick("BTCUSDT", 47000)
	open := eng.OpenOrders(res.Order.UserID)
	if len(open) == 0 {
		t.Fatalf("expected triggered order to be present")
	}
}

func TestSetLeverage(t *testing.T) {
	eng := newTestEngine()
	_ = eng.SetBalance(1, "USDT", 1_000_000_000)

	if err := eng.SetLeverage(1, "BTCUSDT", 8); err != nil {
		t.Fatalf("set leverage failed: %v", err)
	}
	pos := eng.Position(1, "BTCUSDT")
	if pos.Leverage != 8 {
		t.Fatalf("expected leverage 8, got %d", pos.Leverage)
	}
}
