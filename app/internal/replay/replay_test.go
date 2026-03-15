package replay

import (
	"testing"

	"github.com/maxonlinux/meta-terminal-go/internal/clearing"
	"github.com/maxonlinux/meta-terminal-go/internal/oms"
	"github.com/maxonlinux/meta-terminal-go/internal/portfolio"
	"github.com/maxonlinux/meta-terminal-go/internal/registry"
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/events"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/robaho/fixed"
)

func TestReplayOrderAmendedWithPrice(t *testing.T) {
	reg := registry.New()
	reg.SetInstrument("BTCUSDT", &types.Instrument{
		Symbol:     "BTCUSDT",
		BaseAsset:  "BTC",
		QuoteAsset: "USDT",
		MinQty:     types.Quantity(fixed.NewI(1, 0)),
		TickSize:   types.Price(fixed.NewI(1, 0)),
		StepSize:   types.Quantity(fixed.NewI(1, 0)),
	})

	store := oms.NewService()
	port, err := portfolio.New(nil, reg)
	if err != nil {
		t.Fatalf("portfolio: %v", err)
	}
	port.LoadBalance(&types.Balance{
		UserID:    1,
		Asset:     "USDT",
		Available: types.Quantity(fixed.NewI(1000000000, 0)),
		Locked:    types.Quantity(fixed.NewI(0, 0)),
		Margin:    types.Quantity(fixed.NewI(0, 0)),
	})
	clear, err := clearing.New(port, reg)
	if err != nil {
		t.Fatalf("clearing: %v", err)
	}
	player := New(reg, store, port, clear)

	order := &types.Order{
		ID:        1,
		UserID:    1,
		Symbol:    "BTCUSDT",
		Category:  constants.CATEGORY_SPOT,
		Origin:    constants.ORDER_ORIGIN_SYSTEM,
		Side:      constants.ORDER_SIDE_BUY,
		Type:      constants.ORDER_TYPE_LIMIT,
		TIF:       constants.TIF_GTC,
		Status:    constants.ORDER_STATUS_NEW,
		Price:     fixed.NewI(100, 0),
		Quantity:  fixed.NewI(10, 0),
		Filled:    fixed.NewI(0, 0),
		CreatedAt: 1,
		UpdatedAt: 1,
	}

	if err := player.ApplyEvent(events.EncodeOrderPlaced(events.OrderPlacedEvent{Order: order, Instrument: reg.GetInstrument(order.Symbol)})); err != nil {
		t.Fatalf("apply placed: %v", err)
	}
	if err := player.ApplyEvent(events.EncodeOrderAmended(events.OrderAmendedEvent{
		UserID:    order.UserID,
		OrderID:   order.ID,
		NewQty:    types.Quantity(fixed.NewI(9, 0)),
		NewPrice:  types.Price(fixed.NewI(101, 0)),
		Timestamp: 2,
	})); err != nil {
		t.Fatalf("apply amended: %v", err)
	}

	updated, ok := store.GetUserOrder(order.UserID, order.ID)
	if !ok {
		t.Fatalf("order not found")
	}
	if updated.Price.Cmp(types.Price(fixed.NewI(101, 0))) != 0 {
		t.Fatalf("expected price 101, got %s", updated.Price.String())
	}
}
