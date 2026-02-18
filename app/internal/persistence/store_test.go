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

func TestApplyOrderAmendedWithPrice(t *testing.T) {
	reg := registry.New()
	reg.SetInstrument("BTCUSDT", &types.Instrument{
		Symbol:     "BTCUSDT",
		BaseAsset:  "BTC",
		QuoteAsset: "USDT",
		MinQty:     types.Quantity(fixed.NewI(1, 0)),
		MaxQty:     types.Quantity(fixed.NewI(1000000, 0)),
		MinPrice:   types.Price(fixed.NewI(1, 0)),
		MaxPrice:   types.Price(fixed.NewI(1000000, 0)),
		TickSize:   types.Price(fixed.NewI(1, 0)),
		LotSize:    types.Quantity(fixed.NewI(1, 0)),
	})

	store, err := Open(filepath.Join(t.TempDir(), "history"), reg)
	if err != nil {
		t.Fatalf("open history: %v", err)
	}
	defer func() {
		_ = store.Close()
	}()
	store.portfolio.LoadBalance(&types.Balance{
		UserID:    1,
		Asset:     "USDT",
		Available: types.Quantity(fixed.NewI(1000000000, 0)),
		Locked:    types.Quantity(fixed.NewI(0, 0)),
		Margin:    types.Quantity(fixed.NewI(0, 0)),
	})

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
	batch := []events.Event{
		events.EncodeOrderPlaced(order),
		events.EncodeOrderAmended(events.OrderAmendedEvent{
			UserID:    order.UserID,
			OrderID:   order.ID,
			NewQty:    types.Quantity(fixed.NewI(9, 0)),
			NewPrice:  types.Price(fixed.NewI(101, 0)),
			Timestamp: 2,
		}),
	}

	if err := store.Apply(batch); err != nil {
		t.Fatalf("apply: %v", err)
	}

	row := store.db.QueryRow("select price, qty from orders where id = ? and user_id = ?", order.ID, order.UserID)
	var price, qty string
	if err := row.Scan(&price, &qty); err != nil {
		t.Fatalf("scan orders: %v", err)
	}
	if price != "101" {
		t.Fatalf("expected price 101, got %s", price)
	}
	if qty != "9" {
		t.Fatalf("expected qty 9, got %s", qty)
	}

	row = store.db.QueryRow("select price, qty from open_orders where id = ? and user_id = ?", order.ID, order.UserID)
	if err := row.Scan(&price, &qty); err != nil {
		t.Fatalf("scan open_orders: %v", err)
	}
	if price != "101" {
		t.Fatalf("expected open price 101, got %s", price)
	}
	if qty != "9" {
		t.Fatalf("expected open qty 9, got %s", qty)
	}
}
