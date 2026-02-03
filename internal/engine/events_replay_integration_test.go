package engine

import (
	"path/filepath"
	"testing"

	"github.com/maxonlinux/meta-terminal-go/internal/persistence"
	"github.com/maxonlinux/meta-terminal-go/internal/registry"
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/events"
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/outbox"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/robaho/fixed"
)

func TestEventSourcingReplay(t *testing.T) {
	const symbol = "BTCUSDT"
	const baseAsset = "BTC"
	const quoteAsset = "USDT"

	storeDir := filepath.Join(t.TempDir(), "trading")

	reg := registry.New()
	reg.SetInstrument(symbol, &types.Instrument{
		Symbol:     symbol,
		BaseAsset:  baseAsset,
		QuoteAsset: quoteAsset,
		MinQty:     types.Quantity(fixed.NewI(1, 0)),
		MaxQty:     types.Quantity(fixed.NewI(1000000, 0)),
		MinPrice:   types.Price(fixed.NewI(1, 0)),
		MaxPrice:   types.Price(fixed.NewI(1000000, 0)),
		TickSize:   types.Price(fixed.NewI(1, 0)),
		LotSize:    types.Quantity(fixed.NewI(1, 0)),
	})

	historyStore, err := persistence.Open(storeDir, reg)
	if err != nil {
		t.Fatalf("open history: %v", err)
	}
	defer func() {
		_ = historyStore.Close()
	}()

	ob, err := outbox.OpenWithOptions(storeDir, outbox.Options{EventSink: historyStore})
	if err != nil {
		t.Fatalf("open outbox: %v", err)
	}
	ob.Start()

	eng := NewEngine(ob, reg, nil)

	makerID := types.UserID(1)
	takerID := types.UserID(2)
	fundingID := types.UserID(3)

	depositAndApprove(t, eng, makerID, baseAsset, 20)
	depositAndApprove(t, eng, takerID, quoteAsset, 100000)
	depositAndApprove(t, eng, fundingID, quoteAsset, 100)

	eng.OnPriceTick(symbol, types.Price(fixed.NewI(100, 0)))

	placeMaker := &PlaceOrderCmd{Req: &types.PlaceOrderRequest{
		UserID:     makerID,
		Symbol:     symbol,
		Category:   constants.CATEGORY_SPOT,
		Side:       constants.ORDER_SIDE_SELL,
		Type:       constants.ORDER_TYPE_LIMIT,
		TIF:        constants.TIF_GTC,
		Price:      types.Price(fixed.NewI(100, 0)),
		Quantity:   types.Quantity(fixed.NewI(10, 0)),
		ReduceOnly: false,
	}}
	res := eng.Cmd(placeMaker)
	if res.Err != nil {
		t.Fatalf("place maker: %v", res.Err)
	}
	makerOrder := res.Order

	placeTaker := &PlaceOrderCmd{Req: &types.PlaceOrderRequest{
		UserID:   takerID,
		Symbol:   symbol,
		Category: constants.CATEGORY_SPOT,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_MARKET,
		TIF:      constants.TIF_IOC,
		Price:    types.Price(fixed.NewI(100, 0)),
		Quantity: types.Quantity(fixed.NewI(4, 0)),
	}}
	res = eng.Cmd(placeTaker)
	if res.Err != nil {
		t.Fatalf("place taker: %v", res.Err)
	}
	takerOrder := res.Order

	res = eng.Cmd(&CancelOrderCmd{UserID: makerOrder.UserID, OrderID: makerOrder.ID})
	if res.Err != nil {
		t.Fatalf("cancel maker: %v", res.Err)
	}

	placeSecond := &PlaceOrderCmd{Req: &types.PlaceOrderRequest{
		UserID:   makerID,
		Symbol:   symbol,
		Category: constants.CATEGORY_SPOT,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Price:    types.Price(fixed.NewI(110, 0)),
		Quantity: types.Quantity(fixed.NewI(5, 0)),
	}}
	res = eng.Cmd(placeSecond)
	if res.Err != nil {
		t.Fatalf("place second: %v", res.Err)
	}
	secondOrder := res.Order

	res = eng.Cmd(&AmendOrderCmd{UserID: secondOrder.UserID, OrderID: secondOrder.ID, NewQty: types.Quantity(fixed.NewI(4, 0))})
	if res.Err != nil {
		t.Fatalf("amend second: %v", res.Err)
	}

	res = eng.Cmd(&CancelOrderCmd{UserID: secondOrder.UserID, OrderID: secondOrder.ID})
	if res.Err != nil {
		t.Fatalf("cancel second: %v", res.Err)
	}

	res = eng.Cmd(&SetLeverageCmd{UserID: takerID, Symbol: symbol, Leverage: types.Leverage(fixed.NewI(5, 0))})
	if res.Err != nil {
		t.Fatalf("set leverage: %v", res.Err)
	}

	res = eng.Cmd(&CreateWithdrawalCmd{UserID: fundingID, Asset: quoteAsset, Amount: types.Quantity(fixed.NewI(50, 0)), Destination: "wallet", CreatedBy: types.FundingCreatedByUser})
	if res.Err != nil {
		t.Fatalf("create withdrawal: %v", res.Err)
	}
	withdrawal := res.Funding

	res = eng.Cmd(&RejectFundingCmd{FundingID: withdrawal.ID})
	if res.Err != nil {
		t.Fatalf("reject withdrawal: %v", res.Err)
	}

	res = eng.Cmd(&PlaceOrderCmd{Req: &types.PlaceOrderRequest{
		UserID:       makerID,
		Symbol:       symbol,
		Category:     constants.CATEGORY_LINEAR,
		Side:         constants.ORDER_SIDE_BUY,
		Type:         constants.ORDER_TYPE_LIMIT,
		TIF:          constants.TIF_GTC,
		Price:        types.Price(fixed.NewI(100, 0)),
		Quantity:     types.Quantity(fixed.NewI(2, 0)),
		TriggerPrice: types.Price(fixed.NewI(95, 0)),
	}})
	if res.Err != nil {
		t.Fatalf("place conditional: %v", res.Err)
	}
	conditionalOrder := res.Order

	eng.OnPriceTick(symbol, types.Price(fixed.NewI(94, 0)))

	if order, _ := eng.Store().GetUserOrder(conditionalOrder.UserID, conditionalOrder.ID); order == nil || order.Status != constants.ORDER_STATUS_TRIGGERED {
		t.Fatalf("conditional order not triggered")
	}

	makerBTC := *eng.Portfolio().GetBalance(makerID, baseAsset)
	makerUSDT := *eng.Portfolio().GetBalance(makerID, quoteAsset)
	takerBTC := *eng.Portfolio().GetBalance(takerID, baseAsset)
	takerUSDT := *eng.Portfolio().GetBalance(takerID, quoteAsset)
	fundingUSDT := *eng.Portfolio().GetBalance(fundingID, quoteAsset)
	leveraged := eng.Portfolio().GetPosition(takerID, symbol).Leverage

	ob.Stop()
	_ = ob.Close()

	reg2 := registry.New()
	reg2.SetInstrument(symbol, &types.Instrument{
		Symbol:     symbol,
		BaseAsset:  baseAsset,
		QuoteAsset: quoteAsset,
		MinQty:     types.Quantity(fixed.NewI(1, 0)),
		MaxQty:     types.Quantity(fixed.NewI(1000000, 0)),
		MinPrice:   types.Price(fixed.NewI(1, 0)),
		MaxPrice:   types.Price(fixed.NewI(1000000, 0)),
		TickSize:   types.Price(fixed.NewI(1, 0)),
		LotSize:    types.Quantity(fixed.NewI(1, 0)),
	})

	historyStore2, err := persistence.Open(storeDir, reg2)
	if err != nil {
		t.Fatalf("reopen history: %v", err)
	}
	defer func() {
		_ = historyStore2.Close()
	}()

	ob2, err := outbox.OpenWithOptions(storeDir, outbox.Options{EventSink: historyStore2})
	if err != nil {
		t.Fatalf("reopen outbox: %v", err)
	}
	ob2.Start()
	defer func() {
		_ = ob2.Close()
	}()

	eng2 := NewEngine(ob2, reg2, nil)
	if err := historyStore2.LoadCore(eng2.Store(), eng2.Portfolio()); err != nil {
		t.Fatalf("load core: %v", err)
	}
	eng2.RebuildBooks()

	assertBalance(t, eng2.Portfolio().GetBalance(makerID, baseAsset), &makerBTC)
	assertBalance(t, eng2.Portfolio().GetBalance(makerID, quoteAsset), &makerUSDT)
	assertBalance(t, eng2.Portfolio().GetBalance(takerID, baseAsset), &takerBTC)
	assertBalance(t, eng2.Portfolio().GetBalance(takerID, quoteAsset), &takerUSDT)
	assertBalance(t, eng2.Portfolio().GetBalance(fundingID, quoteAsset), &fundingUSDT)

	if pos := eng2.Portfolio().GetPosition(takerID, symbol); math.Cmp(pos.Leverage, leveraged) != 0 {
		t.Fatalf("leverage mismatch: got %s want %s", pos.Leverage.String(), leveraged.String())
	}

	var cancelCount int
	_ = outbox.IterateEvents(storeDir, func(ev events.Event) bool {
		if ev.Type == events.OrderCanceled {
			cancelCount++
		}
		return true
	})

	if order1, _ := eng2.Store().GetUserOrder(makerOrder.UserID, makerOrder.ID); order1 != nil {
		t.Fatalf("order1 should be removed, got %+v (cancelEvents=%d)", order1, cancelCount)
	}
	if order2, _ := eng2.Store().GetUserOrder(takerOrder.UserID, takerOrder.ID); order2 != nil {
		t.Fatalf("order2 should be removed, got %+v", order2)
	}
	if order3, _ := eng2.Store().GetUserOrder(secondOrder.UserID, secondOrder.ID); order3 != nil {
		t.Fatalf("order3 should be removed, got %+v", order3)
	}
	order4, _ := eng2.Store().GetUserOrder(conditionalOrder.UserID, conditionalOrder.ID)
	if order4 == nil || order4.Status != constants.ORDER_STATUS_TRIGGERED {
		t.Fatalf("conditional order status mismatch: %+v", order4)
	}

	book := eng2.ReadBook(constants.CATEGORY_SPOT, symbol)
	if book != nil {
		snapshot := book.Snapshot(10)
		if len(snapshot.Bids) > 0 || len(snapshot.Asks) > 0 {
			t.Fatalf("orderbook not empty after replay: %+v", snapshot)
		}
	}

}

func assertBalance(t *testing.T, got *types.Balance, want *types.Balance) {
	if got == nil || want == nil {
		t.Fatalf("balance missing")
		return
	}
	if math.Cmp(got.Available, want.Available) != 0 || math.Cmp(got.Locked, want.Locked) != 0 || math.Cmp(got.Margin, want.Margin) != 0 {
		t.Fatalf("balance mismatch: got %+v want %+v", got, want)
	}
}

func depositAndApprove(t *testing.T, eng *Engine, userID types.UserID, asset string, amount int64) *types.FundingRequest {
	res := eng.Cmd(&CreateDepositCmd{
		UserID:      userID,
		Asset:       asset,
		Amount:      types.Quantity(fixed.NewI(amount, 0)),
		Destination: "wallet",
		CreatedBy:   types.FundingCreatedByUser,
	})
	if res.Err != nil {
		t.Fatalf("create deposit: %v", res.Err)
	}
	deposit := res.Funding

	res = eng.Cmd(&ApproveFundingCmd{FundingID: deposit.ID})
	if res.Err != nil {
		t.Fatalf("approve deposit: %v", res.Err)
	}
	return deposit
}
