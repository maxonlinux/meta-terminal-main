package engine

import (
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func TestDebugOrderMatching(t *testing.T) {
	eng := newTestEngine()
	eng.AddInstrument("BTCUSDT", constants.CATEGORY_LINEAR, 50000)
	_ = eng.SetBalance(1, "USDT", 1_000_000)
	_ = eng.SetBalance(2, "USDT", 1_000_000)

	buy := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 10,
		Price:    50000,
	}
	res1, err := eng.PlaceOrder(buy)
	if err != nil || res1.Status != constants.ORDER_STATUS_NEW {
		t.Fatalf("limit buy failed")
	}

	sell := &types.OrderInput{
		UserID:   2,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_MARKET,
		TIF:      constants.TIF_IOC,
		Quantity: 10,
	}
	res2, err := eng.PlaceOrder(sell)
	if err != nil || res2.Status != constants.ORDER_STATUS_FILLED {
		t.Fatalf("market sell failed")
	}
	eng.ReleaseResult(res1)
	eng.ReleaseResult(res2)
}

func TestDebugIOCPartialFill(t *testing.T) {
	eng := newTestEngine()
	eng.AddInstrument("BTCUSDT", constants.CATEGORY_LINEAR, 50000)
	_ = eng.SetBalance(1, "USDT", 1_000_000)
	_ = eng.SetBalance(2, "USDT", 1_000_000)

	_, _ = eng.PlaceOrder(&types.OrderInput{UserID: 1, Symbol: "BTCUSDT", Category: constants.CATEGORY_LINEAR, Side: constants.ORDER_SIDE_BUY, Type: constants.ORDER_TYPE_LIMIT, TIF: constants.TIF_GTC, Quantity: 5, Price: 50000})
	res, _ := eng.PlaceOrder(&types.OrderInput{UserID: 2, Symbol: "BTCUSDT", Category: constants.CATEGORY_LINEAR, Side: constants.ORDER_SIDE_SELL, Type: constants.ORDER_TYPE_MARKET, TIF: constants.TIF_IOC, Quantity: 10})
	if res.Status != constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED {
		t.Fatalf("expected partial cancel, got %d", res.Status)
	}
	if res.Filled != 5 {
		t.Fatalf("expected filled 5, got %d", res.Filled)
	}
	eng.ReleaseResult(res)
}

func TestDebugFOKFullFill(t *testing.T) {
	eng := newTestEngine()
	eng.AddInstrument("BTCUSDT", constants.CATEGORY_LINEAR, 50000)
	_ = eng.SetBalance(1, "USDT", 1_000_000)
	_ = eng.SetBalance(2, "USDT", 1_000_000)

	_, _ = eng.PlaceOrder(&types.OrderInput{UserID: 2, Symbol: "BTCUSDT", Category: constants.CATEGORY_LINEAR, Side: constants.ORDER_SIDE_SELL, Type: constants.ORDER_TYPE_LIMIT, TIF: constants.TIF_GTC, Quantity: 10, Price: 50000})
	res, _ := eng.PlaceOrder(&types.OrderInput{UserID: 1, Symbol: "BTCUSDT", Category: constants.CATEGORY_LINEAR, Side: constants.ORDER_SIDE_BUY, Type: constants.ORDER_TYPE_MARKET, TIF: constants.TIF_FOK, Quantity: 10})
	if res.Status != constants.ORDER_STATUS_FILLED {
		t.Fatalf("expected filled, got %d", res.Status)
	}
	eng.ReleaseResult(res)
}

func TestDebugFOKPartialFill(t *testing.T) {
	eng := newTestEngine()
	eng.AddInstrument("BTCUSDT", constants.CATEGORY_LINEAR, 50000)
	_ = eng.SetBalance(1, "USDT", 1_000_000)
	_ = eng.SetBalance(2, "USDT", 1_000_000)

	_, _ = eng.PlaceOrder(&types.OrderInput{UserID: 2, Symbol: "BTCUSDT", Category: constants.CATEGORY_LINEAR, Side: constants.ORDER_SIDE_SELL, Type: constants.ORDER_TYPE_LIMIT, TIF: constants.TIF_GTC, Quantity: 5, Price: 50000})
	res, _ := eng.PlaceOrder(&types.OrderInput{UserID: 1, Symbol: "BTCUSDT", Category: constants.CATEGORY_LINEAR, Side: constants.ORDER_SIDE_BUY, Type: constants.ORDER_TYPE_MARKET, TIF: constants.TIF_FOK, Quantity: 10})
	if res.Status != constants.ORDER_STATUS_CANCELED {
		t.Fatalf("expected canceled, got %d", res.Status)
	}
	if res.Filled != 0 {
		t.Fatalf("expected filled 0, got %d", res.Filled)
	}
	eng.ReleaseResult(res)
}

func TestDebugPostOnlyRejected(t *testing.T) {
	eng := newTestEngine()
	eng.AddInstrument("BTCUSDT", constants.CATEGORY_LINEAR, 50000)
	_ = eng.SetBalance(1, "USDT", 1_000_000)
	_ = eng.SetBalance(2, "USDT", 1_000_000)

	_, _ = eng.PlaceOrder(&types.OrderInput{UserID: 1, Symbol: "BTCUSDT", Category: constants.CATEGORY_LINEAR, Side: constants.ORDER_SIDE_SELL, Type: constants.ORDER_TYPE_LIMIT, TIF: constants.TIF_GTC, Quantity: 10, Price: 50000})
	_, err := eng.PlaceOrder(&types.OrderInput{UserID: 2, Symbol: "BTCUSDT", Category: constants.CATEGORY_LINEAR, Side: constants.ORDER_SIDE_BUY, Type: constants.ORDER_TYPE_LIMIT, TIF: constants.TIF_POST_ONLY, Quantity: 10, Price: 50001})
	if err != ErrPostOnlyWouldCross {
		t.Fatalf("expected post-only cross error")
	}
}

func TestDebugPostOnlyAccepted(t *testing.T) {
	eng := newTestEngine()
	eng.AddInstrument("BTCUSDT", constants.CATEGORY_LINEAR, 50000)
	_ = eng.SetBalance(1, "USDT", 1_000_000)
	_ = eng.SetBalance(2, "USDT", 1_000_000)

	_, _ = eng.PlaceOrder(&types.OrderInput{UserID: 1, Symbol: "BTCUSDT", Category: constants.CATEGORY_LINEAR, Side: constants.ORDER_SIDE_SELL, Type: constants.ORDER_TYPE_LIMIT, TIF: constants.TIF_GTC, Quantity: 10, Price: 50000})
	res, err := eng.PlaceOrder(&types.OrderInput{UserID: 2, Symbol: "BTCUSDT", Category: constants.CATEGORY_LINEAR, Side: constants.ORDER_SIDE_BUY, Type: constants.ORDER_TYPE_LIMIT, TIF: constants.TIF_POST_ONLY, Quantity: 10, Price: 49999})
	if err != nil || res.Status != constants.ORDER_STATUS_NEW {
		t.Fatalf("expected post-only accepted")
	}
	eng.ReleaseResult(res)
}

func TestDebugPostOnlyExactSpread(t *testing.T) {
	eng := newTestEngine()
	eng.AddInstrument("BTCUSDT", constants.CATEGORY_LINEAR, 50000)
	_ = eng.SetBalance(1, "USDT", 1_000_000)
	_ = eng.SetBalance(2, "USDT", 1_000_000)

	_, _ = eng.PlaceOrder(&types.OrderInput{UserID: 1, Symbol: "BTCUSDT", Category: constants.CATEGORY_LINEAR, Side: constants.ORDER_SIDE_SELL, Type: constants.ORDER_TYPE_LIMIT, TIF: constants.TIF_GTC, Quantity: 10, Price: 50000})
	_, err := eng.PlaceOrder(&types.OrderInput{UserID: 2, Symbol: "BTCUSDT", Category: constants.CATEGORY_LINEAR, Side: constants.ORDER_SIDE_BUY, Type: constants.ORDER_TYPE_LIMIT, TIF: constants.TIF_POST_ONLY, Quantity: 10, Price: 50000})
	if err != ErrPostOnlyWouldCross {
		t.Fatalf("expected post-only cross error")
	}
}
