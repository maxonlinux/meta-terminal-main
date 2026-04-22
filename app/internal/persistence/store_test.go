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
		TickSize:   types.Price(fixed.NewI(1, 0)),
		StepSize:   types.Quantity(fixed.NewI(1, 0)),
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
		events.EncodeOrderPlaced(events.OrderPlacedEvent{Order: order, Instrument: reg.GetInstrument(order.Symbol)}),
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

	row = store.db.QueryRow("select price, qty from orders where id = ? and user_id = ?", order.ID, order.UserID)
	if err := row.Scan(&price, &qty); err != nil {
		t.Fatalf("scan orders: %v", err)
	}
	if price != "101" {
		t.Fatalf("expected open price 101, got %s", price)
	}
	if qty != "9" {
		t.Fatalf("expected open qty 9, got %s", qty)
	}
}

func TestApplyTradeAccumulatesOrderFilledOnce(t *testing.T) {
	reg := registry.New()
	inst := &types.Instrument{
		Symbol:     "BTCUSDT",
		BaseAsset:  "BTC",
		QuoteAsset: "USDT",
		MinQty:     types.Quantity(fixed.NewI(1, 0)),
		TickSize:   types.Price(fixed.NewI(1, 0)),
		StepSize:   types.Quantity(fixed.NewI(1, 0)),
	}
	reg.SetInstrument("BTCUSDT", inst)

	store, err := Open(filepath.Join(t.TempDir(), "history"), reg)
	if err != nil {
		t.Fatalf("open history: %v", err)
	}
	defer func() {
		_ = store.Close()
	}()
	store.portfolio.LoadBalance(&types.Balance{UserID: 1, Asset: "USDT", Available: types.Quantity(fixed.NewI(1000000000, 0))})
	store.portfolio.LoadBalance(&types.Balance{UserID: 2, Asset: "BTC", Available: types.Quantity(fixed.NewI(1000000000, 0))})

	makerOrder := &types.Order{
		ID:        101,
		UserID:    1,
		Symbol:    "BTCUSDT",
		Category:  constants.CATEGORY_SPOT,
		Origin:    constants.ORDER_ORIGIN_USER,
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
	takerOrder := &types.Order{
		ID:        102,
		UserID:    2,
		Symbol:    "BTCUSDT",
		Category:  constants.CATEGORY_SPOT,
		Origin:    constants.ORDER_ORIGIN_USER,
		Side:      constants.ORDER_SIDE_SELL,
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
		events.EncodeOrderPlaced(events.OrderPlacedEvent{Order: makerOrder, Instrument: inst}),
		events.EncodeOrderPlaced(events.OrderPlacedEvent{Order: takerOrder, Instrument: inst}),
		events.EncodeTrade(events.TradeEvent{
			TradeID:        1001,
			MakerOrderID:   makerOrder.ID,
			TakerOrderID:   takerOrder.ID,
			MakerUserID:    makerOrder.UserID,
			TakerUserID:    takerOrder.UserID,
			Symbol:         "BTCUSDT",
			Category:       constants.CATEGORY_SPOT,
			Price:          fixed.NewI(100, 0),
			Quantity:       fixed.NewI(1, 0),
			Timestamp:      2,
			TakerSide:      constants.ORDER_SIDE_SELL,
			MakerOrderType: constants.ORDER_TYPE_LIMIT,
			TakerOrderType: constants.ORDER_TYPE_LIMIT,
			Instrument:     inst,
		}),
		events.EncodeTrade(events.TradeEvent{
			TradeID:        1002,
			MakerOrderID:   makerOrder.ID,
			TakerOrderID:   takerOrder.ID,
			MakerUserID:    makerOrder.UserID,
			TakerUserID:    takerOrder.UserID,
			Symbol:         "BTCUSDT",
			Category:       constants.CATEGORY_SPOT,
			Price:          fixed.NewI(100, 0),
			Quantity:       fixed.NewI(1, 0),
			Timestamp:      3,
			TakerSide:      constants.ORDER_SIDE_SELL,
			MakerOrderType: constants.ORDER_TYPE_LIMIT,
			TakerOrderType: constants.ORDER_TYPE_LIMIT,
			Instrument:     inst,
		}),
	}

	if err := store.Apply(batch); err != nil {
		t.Fatalf("apply: %v", err)
	}

	var makerFilled string
	var makerStatus int8
	if err := store.db.QueryRow("select filled, status from orders where id = ? and user_id = ?", makerOrder.ID, makerOrder.UserID).Scan(&makerFilled, &makerStatus); err != nil {
		t.Fatalf("scan maker order: %v", err)
	}
	if makerFilled != "2" {
		t.Fatalf("expected maker filled 2, got %s", makerFilled)
	}
	if makerStatus != constants.ORDER_STATUS_PARTIALLY_FILLED {
		t.Fatalf("expected maker status partially filled, got %d", makerStatus)
	}

	var takerFilled string
	if err := store.db.QueryRow("select filled from orders where id = ? and user_id = ?", takerOrder.ID, takerOrder.UserID).Scan(&takerFilled); err != nil {
		t.Fatalf("scan taker order: %v", err)
	}
	if takerFilled != "2" {
		t.Fatalf("expected taker filled 2, got %s", takerFilled)
	}
}

func TestListFillsSupportsSymbolAndCategoryFilters(t *testing.T) {
	reg := registry.New()
	inst := &types.Instrument{
		Symbol:     "BTCUSDT",
		BaseAsset:  "BTC",
		QuoteAsset: "USDT",
		MinQty:     types.Quantity(fixed.NewI(1, 0)),
		TickSize:   types.Price(fixed.NewI(1, 0)),
		StepSize:   types.Quantity(fixed.NewI(1, 0)),
	}
	reg.SetInstrument("BTCUSDT", inst)

	store, err := Open(filepath.Join(t.TempDir(), "history"), reg)
	if err != nil {
		t.Fatalf("open history: %v", err)
	}
	defer func() {
		_ = store.Close()
	}()
	store.portfolio.LoadBalance(&types.Balance{UserID: 1, Asset: "USDT", Available: types.Quantity(fixed.NewI(1000000000, 0))})
	store.portfolio.LoadBalance(&types.Balance{UserID: 2, Asset: "BTC", Available: types.Quantity(fixed.NewI(1000000000, 0))})

	makerOrder := &types.Order{ID: 201, UserID: 1, Symbol: "BTCUSDT", Category: constants.CATEGORY_SPOT, Origin: constants.ORDER_ORIGIN_USER, Side: constants.ORDER_SIDE_BUY, Type: constants.ORDER_TYPE_LIMIT, TIF: constants.TIF_GTC, Status: constants.ORDER_STATUS_NEW, Price: fixed.NewI(100, 0), Quantity: fixed.NewI(10, 0), CreatedAt: 1, UpdatedAt: 1}
	takerOrder := &types.Order{ID: 202, UserID: 2, Symbol: "BTCUSDT", Category: constants.CATEGORY_SPOT, Origin: constants.ORDER_ORIGIN_USER, Side: constants.ORDER_SIDE_SELL, Type: constants.ORDER_TYPE_LIMIT, TIF: constants.TIF_GTC, Status: constants.ORDER_STATUS_NEW, Price: fixed.NewI(100, 0), Quantity: fixed.NewI(10, 0), CreatedAt: 1, UpdatedAt: 1}

	if err := store.Apply([]events.Event{
		events.EncodeOrderPlaced(events.OrderPlacedEvent{Order: makerOrder, Instrument: inst}),
		events.EncodeOrderPlaced(events.OrderPlacedEvent{Order: takerOrder, Instrument: inst}),
		events.EncodeTrade(events.TradeEvent{TradeID: 2001, MakerOrderID: makerOrder.ID, TakerOrderID: takerOrder.ID, MakerUserID: makerOrder.UserID, TakerUserID: takerOrder.UserID, Symbol: "BTCUSDT", Category: constants.CATEGORY_SPOT, Price: fixed.NewI(100, 0), Quantity: fixed.NewI(1, 0), Timestamp: 2, TakerSide: constants.ORDER_SIDE_SELL, MakerOrderType: constants.ORDER_TYPE_LIMIT, TakerOrderType: constants.ORDER_TYPE_LIMIT, Instrument: inst}),
	}); err != nil {
		t.Fatalf("apply: %v", err)
	}

	category := int8(constants.CATEGORY_SPOT)
	fills, err := store.ListFills(1, "BTCUSDT", &category, 10, 0)
	if err != nil {
		t.Fatalf("list fills with filters: %v", err)
	}
	if len(fills) != 1 {
		t.Fatalf("expected 1 fill, got %d", len(fills))
	}
	if fills[0].Role != "MAKER" {
		t.Fatalf("expected maker role, got %s", fills[0].Role)
	}
	if fills[0].Side != constants.ORDER_SIDE_BUY {
		t.Fatalf("expected maker side BUY, got %d", fills[0].Side)
	}

	takerFills, err := store.ListFills(2, "BTCUSDT", &category, 10, 0)
	if err != nil {
		t.Fatalf("list taker fills with filters: %v", err)
	}
	if len(takerFills) != 1 {
		t.Fatalf("expected 1 taker fill, got %d", len(takerFills))
	}
	if takerFills[0].Role != "TAKER" {
		t.Fatalf("expected taker role, got %s", takerFills[0].Role)
	}
	if takerFills[0].Side != constants.ORDER_SIDE_SELL {
		t.Fatalf("expected taker side SELL, got %d", takerFills[0].Side)
	}
}

func TestCleanupBotDataKeepsBalancesAndPositions(t *testing.T) {
	reg := registry.New()
	reg.SetInstrument("BTCUSDT", &types.Instrument{
		Symbol:     "BTCUSDT",
		BaseAsset:  "BTC",
		QuoteAsset: "USDT",
		MinQty:     types.Quantity(fixed.NewI(1, 0)),
		TickSize:   types.Price(fixed.NewI(1, 0)),
		StepSize:   types.Quantity(fixed.NewI(1, 0)),
	})

	store, err := Open(filepath.Join(t.TempDir(), "history"), reg)
	if err != nil {
		t.Fatalf("open history: %v", err)
	}
	defer func() {
		_ = store.Close()
	}()

	const botID = types.UserID(999999999)

	insertOrder := func(id int64, status int8) {
		t.Helper()
		_, err := store.db.Exec(
			`insert into orders (id, user_id, symbol, category, origin, side, type, tif, status, price, qty, filled, trigger_price, reduce_only, close_on_trigger, stop_order_type, trigger_direction, is_conditional, created_at, updated_at)
			 values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id,
			botID,
			"BTCUSDT",
			constants.CATEGORY_SPOT,
			constants.ORDER_ORIGIN_SYSTEM,
			constants.ORDER_SIDE_BUY,
			constants.ORDER_TYPE_LIMIT,
			constants.TIF_GTC,
			status,
			"100",
			"1",
			"0",
			"0",
			0,
			0,
			0,
			0,
			0,
			1,
			1,
		)
		if err != nil {
			t.Fatalf("insert order: %v", err)
		}
	}

	insertOrder(1001, constants.ORDER_STATUS_FILLED)
	insertOrder(1002, constants.ORDER_STATUS_NEW)

	_, err = store.db.Exec(`insert into balances (user_id, asset, available, locked, margin) values (?, ?, ?, ?, ?)`, botID, "USDT", "1000", "0", "0")
	if err != nil {
		t.Fatalf("insert balance: %v", err)
	}

	_, err = store.db.Exec(
		`insert into positions (user_id, symbol, size, entry_price, exit_price, mode, mm, im, liq_price, leverage, take_profit, stop_loss, tp_order_id, sl_order_id)
		 values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		botID,
		"BTCUSDT",
		"1",
		"100",
		"0",
		0,
		"0",
		"0",
		"0",
		"2",
		"0",
		"0",
		0,
		0,
	)
	if err != nil {
		t.Fatalf("insert position: %v", err)
	}

	_, err = store.db.Exec(
		`insert into trade_fills (id, maker_user_id, taker_user_id, maker_order_id, taker_order_id, symbol, category, maker_order_type, taker_order_type, taker_side, price, qty, ts)
		 values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		2001,
		botID,
		botID,
		1001,
		1002,
		"BTCUSDT",
		constants.CATEGORY_SPOT,
		constants.ORDER_TYPE_LIMIT,
		constants.ORDER_TYPE_LIMIT,
		constants.ORDER_SIDE_SELL,
		"100",
		"1",
		1,
	)
	if err != nil {
		t.Fatalf("insert trade fill: %v", err)
	}

	_, err = store.db.Exec(
		`insert into rpnl_events (id, user_id, order_id, symbol, category, side, price, qty, realized, created_at)
		 values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		3001,
		botID,
		1001,
		"BTCUSDT",
		constants.CATEGORY_SPOT,
		constants.ORDER_SIDE_BUY,
		"100",
		"1",
		"0",
		1,
	)
	if err != nil {
		t.Fatalf("insert rpnl event: %v", err)
	}

	deleted, err := store.CleanupBotData(botID, 2)
	if err != nil {
		t.Fatalf("cleanup bot data: %v", err)
	}
	if deleted != 3 {
		t.Fatalf("expected 3 deleted rows (closed order, trade fill, rpnl), got %d", deleted)
	}

	var ordersCount int
	if err := store.db.QueryRow(`select count(*) from orders where user_id = ?`, botID).Scan(&ordersCount); err != nil {
		t.Fatalf("count orders: %v", err)
	}
	if ordersCount != 1 {
		t.Fatalf("expected 1 active order to remain, got %d", ordersCount)
	}

	var balancesCount int
	if err := store.db.QueryRow(`select count(*) from balances where user_id = ?`, botID).Scan(&balancesCount); err != nil {
		t.Fatalf("count balances: %v", err)
	}
	if balancesCount != 1 {
		t.Fatalf("expected bot balance to remain, got %d", balancesCount)
	}

	var positionsCount int
	if err := store.db.QueryRow(`select count(*) from positions where user_id = ?`, botID).Scan(&positionsCount); err != nil {
		t.Fatalf("count positions: %v", err)
	}
	if positionsCount != 1 {
		t.Fatalf("expected bot position to remain, got %d", positionsCount)
	}
}
