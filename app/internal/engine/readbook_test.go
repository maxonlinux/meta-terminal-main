package engine

import (
	"testing"

	"github.com/maxonlinux/meta-terminal-go/internal/registry"
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/robaho/fixed"
)

func TestReadBookCreatesEmptyBookForKnownInstrument(t *testing.T) {
	reg := registry.New()
	reg.SetInstrument("BTCUSDT", &types.Instrument{
		Symbol:     "BTCUSDT",
		BaseAsset:  "BTC",
		QuoteAsset: "USDT",
		TickSize:   types.Price(fixed.NewI(1, 0)),
		StepSize:   types.Quantity(fixed.NewI(1, 0)),
		MinQty:     types.Quantity(fixed.NewI(1, 0)),
	})

	eng, err := NewEngine(nil, reg, nil)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	book := eng.ReadBook(constants.CATEGORY_SPOT, "BTCUSDT")
	if book == nil {
		t.Fatalf("expected non-nil book for known instrument")
	}

	snapshot := book.Snapshot(10)
	if len(snapshot.Bids) != 0 || len(snapshot.Asks) != 0 {
		t.Fatalf("expected empty snapshot, got bids=%d asks=%d", len(snapshot.Bids), len(snapshot.Asks))
	}
}

func TestReadBookReturnsNilForUnknownInstrument(t *testing.T) {
	eng, err := NewEngine(nil, registry.New(), nil)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	book := eng.ReadBook(constants.CATEGORY_SPOT, "UNKNOWN")
	if book != nil {
		t.Fatalf("expected nil book for unknown instrument")
	}
}
