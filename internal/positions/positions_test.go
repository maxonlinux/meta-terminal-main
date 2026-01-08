package positions

import (
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func TestUpdateOpenLong(t *testing.T) {
	p := New("BTCUSDT")
	p.Update(10, 100, constants.ORDER_SIDE_BUY, 2)
	if p.Size != 10 {
		t.Fatalf("expected size 10, got %d", p.Size)
	}
	if p.EntryPrice != 100 {
		t.Fatalf("expected entry 100, got %d", p.EntryPrice)
	}
	if p.Side != constants.ORDER_SIDE_BUY {
		t.Fatalf("expected side BUY")
	}
}

func TestUpdateOpenShort(t *testing.T) {
	p := New("BTCUSDT")
	p.Update(10, 100, constants.ORDER_SIDE_SELL, 2)
	if p.Size != 10 {
		t.Fatalf("expected size 10")
	}
	if p.Side != constants.ORDER_SIDE_SELL {
		t.Fatalf("expected side SELL")
	}
	if p.EntryPrice != 100 {
		t.Fatalf("expected entry 100")
	}
}

func TestUpdateAddToLong(t *testing.T) {
	p := New("BTCUSDT")
	p.Update(10, 100, constants.ORDER_SIDE_BUY, 2)
	p.Update(5, 110, constants.ORDER_SIDE_BUY, 2)
	if p.Size != 15 {
		t.Fatalf("expected size 15, got %d", p.Size)
	}
	if p.EntryPrice != types.Price(103) && p.EntryPrice != types.Price(104) {
		// weighted average with safemath should be stable; accept rounding by big int
		t.Fatalf("unexpected entry price %d", p.EntryPrice)
	}
}

func TestUpdateCloseLong(t *testing.T) {
	p := New("BTCUSDT")
	p.Update(10, 100, constants.ORDER_SIDE_BUY, 2)
	p.Update(5, 120, constants.ORDER_SIDE_SELL, 2)
	if p.Size != 5 {
		t.Fatalf("expected size 5, got %d", p.Size)
	}
}

func TestUpdateCloseFully(t *testing.T) {
	p := New("BTCUSDT")
	p.Update(10, 100, constants.ORDER_SIDE_BUY, 2)
	p.Update(10, 120, constants.ORDER_SIDE_SELL, 2)
	if p.Size != 0 {
		t.Fatalf("expected size 0, got %d", p.Size)
	}
}

func TestUpdateReverse(t *testing.T) {
	p := New("BTCUSDT")
	p.Update(10, 100, constants.ORDER_SIDE_SELL, 2)
	p.Update(15, 90, constants.ORDER_SIDE_BUY, 2)
	if p.Size != 5 {
		t.Fatalf("expected size 5, got %d", p.Size)
	}
	if p.Side != constants.ORDER_SIDE_BUY {
		t.Fatalf("expected side BUY")
	}
}

func TestSetLeverage(t *testing.T) {
	p := New("BTCUSDT")
	p.Update(10, 100, constants.ORDER_SIDE_BUY, 2)
	p.SetLeverage(5)
	if p.Leverage != 5 {
		t.Fatalf("expected leverage 5")
	}
}
