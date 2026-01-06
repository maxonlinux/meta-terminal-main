package orderbook

import (
	"testing"
	"time"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func TestPlaceLimitOrderGTC(t *testing.T) {
	s := state.New()
	symbol := types.SymbolID(1)
	ob := New(symbol, constants.CATEGORY_SPOT, s)

	order := &types.Order{
		ID:       1,
		UserID:   1,
		Symbol:   symbol,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Price:    100,
		Quantity: 10,
		Status:   constants.ORDER_STATUS_NEW,
	}

	trades, remaining, err := ob.PlaceOrder(order)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(trades) != 0 {
		t.Errorf("expected 0 trades, got %d", len(trades))
	}

	if remaining != 10 {
		t.Errorf("expected remaining 10, got %d", remaining)
	}

	bids := ob.GetBids()
	if len(bids) != 1 {
		t.Fatalf("expected 1 bid level, got %d", len(bids))
	}
	if bids[0].Price != 100 {
		t.Errorf("expected bid price 100, got %d", bids[0].Price)
	}
	if bids[0].Quantity != 10 {
		t.Errorf("expected bid quantity 10, got %d", bids[0].Quantity)
	}
}

func TestMatchLimitOrder(t *testing.T) {
	s := state.New()
	symbol := types.SymbolID(1)
	ob := New(symbol, constants.CATEGORY_SPOT, s)

	sellerOrder := &types.Order{
		ID:       1,
		UserID:   1,
		Symbol:   symbol,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Price:    100,
		Quantity: 10,
		Status:   constants.ORDER_STATUS_NEW,
	}

	_, _, _ = ob.PlaceOrder(sellerOrder)

	buyerOrder := &types.Order{
		ID:       2,
		UserID:   2,
		Symbol:   symbol,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Price:    100,
		Quantity: 5,
		Status:   constants.ORDER_STATUS_NEW,
	}

	trades, remaining, err := ob.PlaceOrder(buyerOrder)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(trades))
	}

	if remaining != 0 {
		t.Errorf("expected remaining 0, got %d", remaining)
	}

	trade := trades[0]
	if trade.Quantity != 5 {
		t.Errorf("expected trade quantity 5, got %d", trade.Quantity)
	}
	if trade.Price != 100 {
		t.Errorf("expected trade price 100, got %d", trade.Price)
	}
	if trade.SellerID != 1 {
		t.Errorf("expected seller id 1, got %d", trade.SellerID)
	}
	if trade.BuyerID != 2 {
		t.Errorf("expected buyer id 2, got %d", trade.BuyerID)
	}

	asks := ob.GetAsks()
	if len(asks) != 1 {
		t.Fatalf("expected 1 ask level, got %d", len(asks))
	}
	if asks[0].Quantity != 5 {
		t.Errorf("expected ask quantity 5, got %d", asks[0].Quantity)
	}
}

func TestMarketOrderMatch(t *testing.T) {
	s := state.New()
	symbol := types.SymbolID(1)
	ob := New(symbol, constants.CATEGORY_SPOT, s)

	sellerOrder := &types.Order{
		ID:       1,
		UserID:   1,
		Symbol:   symbol,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Price:    100,
		Quantity: 10,
		Status:   constants.ORDER_STATUS_NEW,
	}

	_, _, _ = ob.PlaceOrder(sellerOrder)

	buyerOrder := &types.Order{
		ID:       2,
		UserID:   2,
		Symbol:   symbol,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_MARKET,
		Price:    100,
		Quantity: 5,
		Status:   constants.ORDER_STATUS_NEW,
	}

	trades, remaining, err := ob.PlaceOrder(buyerOrder)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(trades))
	}

	if remaining != 0 {
		t.Errorf("expected remaining 0, got %d", remaining)
	}
}

func TestFOKPartialFillRejection(t *testing.T) {
	s := state.New()
	symbol := types.SymbolID(1)
	ob := New(symbol, constants.CATEGORY_SPOT, s)

	sellerOrder := &types.Order{
		ID:       1,
		UserID:   1,
		Symbol:   symbol,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Price:    100,
		Quantity: 5,
		Status:   constants.ORDER_STATUS_NEW,
	}

	_, _, _ = ob.PlaceOrder(sellerOrder)

	buyerOrder := &types.Order{
		ID:       2,
		UserID:   2,
		Symbol:   symbol,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_FOK,
		Price:    100,
		Quantity: 10,
		Status:   constants.ORDER_STATUS_NEW,
	}

	trades, remaining, err := ob.PlaceOrder(buyerOrder)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(trades) != 0 {
		t.Errorf("expected 0 trades after FOK rejection, got %d", len(trades))
	}

	if remaining != 10 {
		t.Errorf("expected remaining 10, got %d", remaining)
	}

	if buyerOrder.Status != constants.ORDER_STATUS_CANCELED {
		t.Errorf("expected FOK order canceled, got status %d", buyerOrder.Status)
	}

	bids := ob.GetBids()
	if len(bids) != 0 {
		t.Error("expected no bids after FOK rejection")
	}
}

func TestIOCPartialFillAcceptance(t *testing.T) {
	s := state.New()
	symbol := types.SymbolID(1)
	ob := New(symbol, constants.CATEGORY_SPOT, s)

	sellerOrder := &types.Order{
		ID:       1,
		UserID:   1,
		Symbol:   symbol,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Price:    100,
		Quantity: 5,
		Status:   constants.ORDER_STATUS_NEW,
	}

	_, _, _ = ob.PlaceOrder(sellerOrder)

	buyerOrder := &types.Order{
		ID:       2,
		UserID:   2,
		Symbol:   symbol,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_IOC,
		Price:    100,
		Quantity: 10,
		Status:   constants.ORDER_STATUS_NEW,
	}

	trades, remaining, err := ob.PlaceOrder(buyerOrder)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(trades))
	}

	if remaining != 5 {
		t.Errorf("expected remaining 5, got %d", remaining)
	}

	if buyerOrder.Status != constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED {
		t.Errorf("expected PARTIALLY_FILLED_CANCELED, got %d", buyerOrder.Status)
	}
}

func TestPostOnlyRejection(t *testing.T) {
	s := state.New()
	symbol := types.SymbolID(1)
	ob := New(symbol, constants.CATEGORY_SPOT, s)

	sellerOrder := &types.Order{
		ID:       1,
		UserID:   1,
		Symbol:   symbol,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Price:    100,
		Quantity: 10,
		Status:   constants.ORDER_STATUS_NEW,
	}

	_, _, _ = ob.PlaceOrder(sellerOrder)

	buyerOrder := &types.Order{
		ID:       2,
		UserID:   2,
		Symbol:   symbol,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_POST_ONLY,
		Price:    100,
		Quantity: 10,
		Status:   constants.ORDER_STATUS_NEW,
	}

	_, _, err := ob.PlaceOrder(buyerOrder)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if buyerOrder.Status != constants.ORDER_STATUS_CANCELED {
		t.Errorf("expected CANCELED, got %d", buyerOrder.Status)
	}

	bids := ob.GetBids()
	if len(bids) != 0 {
		t.Error("expected no bids after POST_ONLY rejection")
	}
}

func TestPriceTimePriority(t *testing.T) {
	s := state.New()
	symbol := types.SymbolID(1)
	ob := New(symbol, constants.CATEGORY_SPOT, s)

	order1 := &types.Order{
		ID:        1,
		UserID:    1,
		Symbol:    symbol,
		Side:      constants.ORDER_SIDE_BUY,
		Type:      constants.ORDER_TYPE_LIMIT,
		TIF:       constants.TIF_GTC,
		Price:     100,
		Quantity:  10,
		Status:    constants.ORDER_STATUS_NEW,
		CreatedAt: time.Now(),
	}

	order2 := &types.Order{
		ID:        2,
		UserID:    2,
		Symbol:    symbol,
		Side:      constants.ORDER_SIDE_BUY,
		Type:      constants.ORDER_TYPE_LIMIT,
		TIF:       constants.TIF_GTC,
		Price:     101,
		Quantity:  5,
		Status:    constants.ORDER_STATUS_NEW,
		CreatedAt: time.Now().Add(time.Second),
	}

	_, _, _ = ob.PlaceOrder(order1)
	_, _, _ = ob.PlaceOrder(order2)

	bids := ob.GetBids()
	if len(bids) != 2 {
		t.Fatalf("expected 2 bid levels, got %d", len(bids))
	}

	if bids[0].Price != 101 {
		t.Errorf("expected best bid 101, got %d", bids[0].Price)
	}
}

func TestMultipleFills(t *testing.T) {
	s := state.New()
	symbol := types.SymbolID(1)
	ob := New(symbol, constants.CATEGORY_SPOT, s)

	seller1 := &types.Order{
		ID:       1,
		UserID:   1,
		Symbol:   symbol,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Price:    100,
		Quantity: 5,
		Status:   constants.ORDER_STATUS_NEW,
	}

	seller2 := &types.Order{
		ID:       2,
		UserID:   2,
		Symbol:   symbol,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Price:    100,
		Quantity: 5,
		Status:   constants.ORDER_STATUS_NEW,
	}

	_, _, _ = ob.PlaceOrder(seller1)
	_, _, _ = ob.PlaceOrder(seller2)

	buyer := &types.Order{
		ID:       3,
		UserID:   3,
		Symbol:   symbol,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Price:    100,
		Quantity: 8,
		Status:   constants.ORDER_STATUS_NEW,
	}

	trades, remaining, err := ob.PlaceOrder(buyer)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(trades) != 2 {
		t.Fatalf("expected 2 trades, got %d", len(trades))
	}

	totalFilled := types.Quantity(0)
	for _, trade := range trades {
		totalFilled += trade.Quantity
	}

	if totalFilled != 8 {
		t.Errorf("expected total filled 8, got %d", totalFilled)
	}

	if remaining != 0 {
		t.Errorf("expected remaining 0, got %d", remaining)
	}
}
