package persistence

import (
	"path/filepath"
	"testing"

	"github.com/maxonlinux/meta-terminal-go/internal/registry"
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/events"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/robaho/fixed"
)

func BenchmarkHistoryApply(b *testing.B) {
	reg := registry.New()
	reg.SetInstrument("BTCUSDT", &types.Instrument{
		Symbol:     "BTCUSDT",
		BaseAsset:  "BTC",
		QuoteAsset: "USDT",
		MinQty:     types.Quantity(fixed.NewI(1, 0)),
		TickSize:   types.Price(fixed.NewI(1, 0)),
		StepSize:   types.Quantity(fixed.NewI(1, 0)),
	})
	inst := reg.GetInstrument("BTCUSDT")

	store, err := Open(filepath.Join(b.TempDir(), "history"), reg)
	if err != nil {
		b.Fatalf("open history: %v", err)
	}
	b.Cleanup(func() {
		_ = store.Close()
	})
	if _, err := store.db.Exec("pragma synchronous=normal"); err != nil {
		b.Fatalf("pragma sync: %v", err)
	}
	if _, err := store.db.Exec("pragma temp_store=memory"); err != nil {
		b.Fatalf("pragma temp store: %v", err)
	}

	order := &types.Order{
		ID:        1,
		UserID:    1,
		Symbol:    "BTCUSDT",
		Category:  constants.CATEGORY_SPOT,
		Origin:    constants.ORDER_ORIGIN_USER,
		Side:      constants.ORDER_SIDE_BUY,
		Type:      constants.ORDER_TYPE_LIMIT,
		TIF:       constants.TIF_GTC,
		Status:    constants.ORDER_STATUS_NEW,
		Price:     fixed.NewI(100, 0),
		Quantity:  fixed.NewI(1, 0),
		Filled:    fixed.NewI(0, 0),
		CreatedAt: 1,
		UpdatedAt: 1,
	}
	batch := make([]events.Event, 0, 1000)
	for i := 0; i < cap(batch); i++ {
		ord := *order
		ord.ID = types.OrderID(i + 1)
		batch = append(batch, events.EncodeOrderPlaced(events.OrderPlacedEvent{Order: &ord, Instrument: inst}))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.portfolio.LoadBalance(&types.Balance{
			UserID:    1,
			Asset:     "USDT",
			Available: types.Quantity(fixed.NewI(1000000000, 0)),
			Locked:    types.Quantity(fixed.NewI(0, 0)),
			Margin:    types.Quantity(fixed.NewI(0, 0)),
		})
		if err := store.Apply(batch); err != nil {
			b.Fatalf("apply: %v", err)
		}
	}
}

func BenchmarkHistoryApplyDefault(b *testing.B) {
	reg := registry.New()
	reg.SetInstrument("BTCUSDT", &types.Instrument{
		Symbol:     "BTCUSDT",
		BaseAsset:  "BTC",
		QuoteAsset: "USDT",
		MinQty:     types.Quantity(fixed.NewI(1, 0)),
		TickSize:   types.Price(fixed.NewI(1, 0)),
		StepSize:   types.Quantity(fixed.NewI(1, 0)),
	})
	inst := reg.GetInstrument("BTCUSDT")

	store, err := Open(filepath.Join(b.TempDir(), "history"), reg)
	if err != nil {
		b.Fatalf("open history: %v", err)
	}
	b.Cleanup(func() {
		_ = store.Close()
	})

	order := &types.Order{
		ID:        1,
		UserID:    1,
		Symbol:    "BTCUSDT",
		Category:  constants.CATEGORY_SPOT,
		Origin:    constants.ORDER_ORIGIN_USER,
		Side:      constants.ORDER_SIDE_BUY,
		Type:      constants.ORDER_TYPE_LIMIT,
		TIF:       constants.TIF_GTC,
		Status:    constants.ORDER_STATUS_NEW,
		Price:     fixed.NewI(100, 0),
		Quantity:  fixed.NewI(1, 0),
		Filled:    fixed.NewI(0, 0),
		CreatedAt: 1,
		UpdatedAt: 1,
	}
	batch := make([]events.Event, 0, 1000)
	for i := 0; i < cap(batch); i++ {
		ord := *order
		ord.ID = types.OrderID(i + 1)
		batch = append(batch, events.EncodeOrderPlaced(events.OrderPlacedEvent{Order: &ord, Instrument: inst}))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.portfolio.LoadBalance(&types.Balance{
			UserID:    1,
			Asset:     "USDT",
			Available: types.Quantity(fixed.NewI(1000000000, 0)),
			Locked:    types.Quantity(fixed.NewI(0, 0)),
			Margin:    types.Quantity(fixed.NewI(0, 0)),
		})
		if err := store.Apply(batch); err != nil {
			b.Fatalf("apply: %v", err)
		}
	}
}
