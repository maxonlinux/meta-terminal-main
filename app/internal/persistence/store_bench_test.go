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

const tradeBurstSize = 1000
const tradeMixedMarkerEvery = 10

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

func BenchmarkHistoryApplyTradeBurst(b *testing.B) {
	benchmarkHistoryApplyTradeBurst(b, true)
}

func BenchmarkHistoryApplyTradeBurstNoOrderFillCoalesce(b *testing.B) {
	benchmarkHistoryApplyTradeBurst(b, false)
}

func benchmarkHistoryApplyTradeBurst(b *testing.B, coalesceOrderProgress bool) {
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
	store.coalesceOrderProgress = coalesceOrderProgress

	store.portfolio.LoadBalance(&types.Balance{UserID: 1, Asset: "USDT", Available: types.Quantity(fixed.NewI(1000000000, 0))})
	store.portfolio.LoadBalance(&types.Balance{UserID: 2, Asset: "BTC", Available: types.Quantity(fixed.NewI(1000000000, 0))})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		store.portfolio.LoadBalance(&types.Balance{UserID: 1, Asset: "USDT", Available: types.Quantity(fixed.NewI(1000000000, 0))})
		store.portfolio.LoadBalance(&types.Balance{UserID: 2, Asset: "BTC", Available: types.Quantity(fixed.NewI(1000000000, 0))})
		batch := buildTradeBurstBatch(inst, i)
		b.StartTimer()
		if err := store.Apply(batch); err != nil {
			b.Fatalf("apply: %v", err)
		}
	}
}

func BenchmarkHistoryApplyTradeBurstMixedBoundaries(b *testing.B) {
	benchmarkHistoryApplyTradeBurstMixedBoundaries(b, true)
}

func BenchmarkHistoryApplyTradeBurstMixedBoundariesNoOrderFillCoalesce(b *testing.B) {
	benchmarkHistoryApplyTradeBurstMixedBoundaries(b, false)
}

func benchmarkHistoryApplyTradeBurstMixedBoundaries(b *testing.B, coalesceOrderProgress bool) {
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
	store.coalesceOrderProgress = coalesceOrderProgress

	store.portfolio.LoadBalance(&types.Balance{UserID: 1, Asset: "USDT", Available: types.Quantity(fixed.NewI(1000000000, 0))})
	store.portfolio.LoadBalance(&types.Balance{UserID: 2, Asset: "BTC", Available: types.Quantity(fixed.NewI(1000000000, 0))})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		store.portfolio.LoadBalance(&types.Balance{UserID: 1, Asset: "USDT", Available: types.Quantity(fixed.NewI(1000000000, 0))})
		store.portfolio.LoadBalance(&types.Balance{UserID: 2, Asset: "BTC", Available: types.Quantity(fixed.NewI(1000000000, 0))})
		batch := buildTradeBurstMixedBatch(inst, i)
		b.StartTimer()
		if err := store.Apply(batch); err != nil {
			b.Fatalf("apply: %v", err)
		}
	}
}

func buildTradeBurstBatch(inst *types.Instrument, iteration int) []events.Event {
	batch := make([]events.Event, 0, tradeBurstSize+2)
	baseOrderID := types.OrderID(1000000 + iteration*10)
	baseTradeID := types.TradeID(100000000 + iteration*tradeBurstSize)

	makerOrder := &types.Order{
		ID:        baseOrderID,
		UserID:    1,
		Symbol:    "BTCUSDT",
		Category:  constants.CATEGORY_SPOT,
		Origin:    constants.ORDER_ORIGIN_USER,
		Side:      constants.ORDER_SIDE_BUY,
		Type:      constants.ORDER_TYPE_LIMIT,
		TIF:       constants.TIF_GTC,
		Status:    constants.ORDER_STATUS_NEW,
		Price:     fixed.NewI(100, 0),
		Quantity:  fixed.NewI(1000000, 0),
		Filled:    fixed.NewI(0, 0),
		CreatedAt: uint64(iteration + 1),
		UpdatedAt: uint64(iteration + 1),
	}
	takerOrder := &types.Order{
		ID:        baseOrderID + 1,
		UserID:    2,
		Symbol:    "BTCUSDT",
		Category:  constants.CATEGORY_SPOT,
		Origin:    constants.ORDER_ORIGIN_USER,
		Side:      constants.ORDER_SIDE_SELL,
		Type:      constants.ORDER_TYPE_LIMIT,
		TIF:       constants.TIF_GTC,
		Status:    constants.ORDER_STATUS_NEW,
		Price:     fixed.NewI(100, 0),
		Quantity:  fixed.NewI(1000000, 0),
		Filled:    fixed.NewI(0, 0),
		CreatedAt: uint64(iteration + 1),
		UpdatedAt: uint64(iteration + 1),
	}

	batch = append(batch, events.EncodeOrderPlaced(events.OrderPlacedEvent{Order: makerOrder, Instrument: inst}))
	batch = append(batch, events.EncodeOrderPlaced(events.OrderPlacedEvent{Order: takerOrder, Instrument: inst}))

	for i := 0; i < tradeBurstSize; i++ {
		batch = append(batch, events.EncodeTrade(events.TradeEvent{
			TradeID:        baseTradeID + types.TradeID(i),
			MakerOrderID:   makerOrder.ID,
			TakerOrderID:   takerOrder.ID,
			MakerUserID:    makerOrder.UserID,
			TakerUserID:    takerOrder.UserID,
			Symbol:         "BTCUSDT",
			Category:       constants.CATEGORY_SPOT,
			Price:          fixed.NewI(100, 0),
			Quantity:       fixed.NewI(1, 0),
			Timestamp:      uint64(10 + i + iteration*tradeBurstSize),
			TakerSide:      constants.ORDER_SIDE_SELL,
			MakerOrderType: constants.ORDER_TYPE_LIMIT,
			TakerOrderType: constants.ORDER_TYPE_LIMIT,
			Instrument:     inst,
		}))
	}

	return batch
}

func buildTradeBurstMixedBatch(inst *types.Instrument, iteration int) []events.Event {
	batch := make([]events.Event, 0, tradeBurstSize+2+tradeBurstSize/tradeMixedMarkerEvery)
	baseOrderID := types.OrderID(2000000 + iteration*10)
	baseTradeID := types.TradeID(200000000 + iteration*tradeBurstSize)

	makerOrder := &types.Order{
		ID:        baseOrderID,
		UserID:    1,
		Symbol:    "BTCUSDT",
		Category:  constants.CATEGORY_SPOT,
		Origin:    constants.ORDER_ORIGIN_USER,
		Side:      constants.ORDER_SIDE_BUY,
		Type:      constants.ORDER_TYPE_LIMIT,
		TIF:       constants.TIF_GTC,
		Status:    constants.ORDER_STATUS_NEW,
		Price:     fixed.NewI(100, 0),
		Quantity:  fixed.NewI(1000000, 0),
		Filled:    fixed.NewI(0, 0),
		CreatedAt: uint64(iteration + 1),
		UpdatedAt: uint64(iteration + 1),
	}
	takerOrder := &types.Order{
		ID:        baseOrderID + 1,
		UserID:    2,
		Symbol:    "BTCUSDT",
		Category:  constants.CATEGORY_SPOT,
		Origin:    constants.ORDER_ORIGIN_USER,
		Side:      constants.ORDER_SIDE_SELL,
		Type:      constants.ORDER_TYPE_LIMIT,
		TIF:       constants.TIF_GTC,
		Status:    constants.ORDER_STATUS_NEW,
		Price:     fixed.NewI(100, 0),
		Quantity:  fixed.NewI(1000000, 0),
		Filled:    fixed.NewI(0, 0),
		CreatedAt: uint64(iteration + 1),
		UpdatedAt: uint64(iteration + 1),
	}

	batch = append(batch, events.EncodeOrderPlaced(events.OrderPlacedEvent{Order: makerOrder, Instrument: inst}))
	batch = append(batch, events.EncodeOrderPlaced(events.OrderPlacedEvent{Order: takerOrder, Instrument: inst}))

	for i := 0; i < tradeBurstSize; i++ {
		ts := uint64(10 + i + iteration*tradeBurstSize)
		batch = append(batch, events.EncodeTrade(events.TradeEvent{
			TradeID:        baseTradeID + types.TradeID(i),
			MakerOrderID:   makerOrder.ID,
			TakerOrderID:   takerOrder.ID,
			MakerUserID:    makerOrder.UserID,
			TakerUserID:    takerOrder.UserID,
			Symbol:         "BTCUSDT",
			Category:       constants.CATEGORY_SPOT,
			Price:          fixed.NewI(100, 0),
			Quantity:       fixed.NewI(1, 0),
			Timestamp:      ts,
			TakerSide:      constants.ORDER_SIDE_SELL,
			MakerOrderType: constants.ORDER_TYPE_LIMIT,
			TakerOrderType: constants.ORDER_TYPE_LIMIT,
			Instrument:     inst,
		}))
		if i%tradeMixedMarkerEvery == tradeMixedMarkerEvery-1 {
			batch = append(batch, events.EncodeRPNL(events.RPNLEvent{
				UserID:    makerOrder.UserID,
				OrderID:   makerOrder.ID,
				Symbol:    makerOrder.Symbol,
				Category:  makerOrder.Category,
				Side:      makerOrder.Side,
				Price:     fixed.NewI(100, 0),
				Quantity:  fixed.NewI(1, 0),
				Realized:  fixed.NewI(1, 0),
				Timestamp: ts + 1,
			}))
		}
	}

	return batch
}
