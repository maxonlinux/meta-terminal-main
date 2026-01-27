package engine

import (
	"path/filepath"
	"testing"

	"github.com/maxonlinux/meta-terminal-go/internal/registry"
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/outbox"
	"github.com/maxonlinux/meta-terminal-go/pkg/persistence"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/robaho/fixed"
)

func TestEventSourcingReplay(t *testing.T) {
	const symbol = "BTCUSDT"
	const baseAsset = "BTC"
	const quoteAsset = "USDT"

	storeDir := filepath.Join(t.TempDir(), "trading")
	store, err := persistence.Open(storeDir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	ob, err := outbox.Open(filepath.Dir(storeDir), store.DB())
	if err != nil {
		_ = store.Close()
		t.Fatalf("open outbox: %v", err)
	}
	ob.Start()

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

	eng := NewEngine(store, ob, reg, nil)

	makerID := types.UserID(1)
	takerID := types.UserID(2)
	fundingID := types.UserID(3)

	depositAndApprove(t, eng, makerID, baseAsset, 20)
	depositAndApprove(t, eng, takerID, quoteAsset, 100000)
	deposit := depositAndApprove(t, eng, fundingID, quoteAsset, 100)

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
	_ = store.Close()

	store2, err := persistence.Open(storeDir)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	defer func() {
		_ = store2.Close()
	}()

	ob2, err := outbox.Open(filepath.Dir(storeDir), store2.DB())
	if err != nil {
		_ = store2.Close()
		t.Fatalf("reopen outbox: %v", err)
	}
	ob2.Start()
	defer func() {
		_ = ob2.Close()
	}()

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

	eng2 := NewEngine(store2, ob2, reg2, nil)

	assertBalance(t, eng2.Portfolio().GetBalance(makerID, baseAsset), &makerBTC)
	assertBalance(t, eng2.Portfolio().GetBalance(makerID, quoteAsset), &makerUSDT)
	assertBalance(t, eng2.Portfolio().GetBalance(takerID, baseAsset), &takerBTC)
	assertBalance(t, eng2.Portfolio().GetBalance(takerID, quoteAsset), &takerUSDT)
	assertBalance(t, eng2.Portfolio().GetBalance(fundingID, quoteAsset), &fundingUSDT)

	if pos := eng2.Portfolio().GetPosition(takerID, symbol); math.Cmp(pos.Leverage, leveraged) != 0 {
		t.Fatalf("leverage mismatch: got %s want %s", pos.Leverage.String(), leveraged.String())
	}

	order1, _ := eng2.Store().GetUserOrder(makerOrder.UserID, makerOrder.ID)
	if order1 == nil || order1.Status != constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED {
		t.Fatalf("order1 status mismatch: %+v", order1)
	}
	order2, _ := eng2.Store().GetUserOrder(takerOrder.UserID, takerOrder.ID)
	if order2 == nil || order2.Status != constants.ORDER_STATUS_FILLED {
		t.Fatalf("order2 status mismatch: %+v", order2)
	}
	order3, _ := eng2.Store().GetUserOrder(secondOrder.UserID, secondOrder.ID)
	if order3 == nil || order3.Status != constants.ORDER_STATUS_CANCELED {
		t.Fatalf("order3 status mismatch: %+v", order3)
	}
	order4, _ := eng2.Store().GetUserOrder(conditionalOrder.UserID, conditionalOrder.ID)
	if order4 == nil || order4.Status != constants.ORDER_STATUS_TRIGGERED {
		t.Fatalf("conditional order status mismatch: %+v", order4)
	}

	if funding := eng2.Portfolio().Fundings[deposit.ID]; funding == nil || funding.Status != types.FundingStatusCompleted {
		t.Fatalf("deposit status mismatch: %+v", funding)
	}
	if funding := eng2.Portfolio().Fundings[withdrawal.ID]; funding == nil || funding.Status != types.FundingStatusCanceled {
		t.Fatalf("withdrawal status mismatch: %+v", funding)
	}

	book := eng2.ReadBook(constants.CATEGORY_SPOT, symbol)
	if book != nil {
		snapshot := book.Snapshot(10)
		if len(snapshot.Bids) > 0 || len(snapshot.Asks) > 0 {
			t.Fatalf("orderbook not empty after replay: %+v", snapshot)
		}
	}

	var eventCount int
	_ = store2.IterateEvents(func(_, _ []byte) bool {
		eventCount++
		return true
	})
	if eventCount != 18 {
		t.Fatalf("event count mismatch: got %d want %d", eventCount, 18)
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
