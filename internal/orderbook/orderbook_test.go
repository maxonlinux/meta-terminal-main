package orderbook

import (
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func TestAddOrder(t *testing.T) {
	ob := New()
	ss := &state.OrderBookState{
		BidIndex: make(map[types.Price]*state.PriceLevel),
		AskIndex: make(map[types.Price]*state.PriceLevel),
	}

	order := &types.Order{
		ID:       1,
		UserID:   1,
		Symbol:   1,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Price:    100,
		Quantity: 10,
		Status:   constants.ORDER_STATUS_NEW,
	}

	ob.AddOrder(ss, order)

	level := ss.BidIndex[100]
	if level == nil {
		t.Fatal("expected BidIndex[100] to be set")
	}
	if level.Price != 100 {
		t.Errorf("expected bid price 100, got %d", level.Price)
	}
	if level.Quantity != 10 {
		t.Errorf("expected bid quantity 10, got %d", level.Quantity)
	}
	if level.Orders.Len() != 1 {
		t.Errorf("expected 1 order in heap, got %d", level.Orders.Len())
	}
	if ss.BestBid != level {
		t.Error("expected BestBid to point to the level")
	}
}

func TestAddOrderMultipleOrders(t *testing.T) {
	ob := New()
	ss := &state.OrderBookState{
		BidIndex: make(map[types.Price]*state.PriceLevel),
		AskIndex: make(map[types.Price]*state.PriceLevel),
	}

	order1 := &types.Order{
		ID:       1,
		UserID:   1,
		Symbol:   1,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		Price:    100,
		Quantity: 10,
	}
	order2 := &types.Order{
		ID:       2,
		UserID:   2,
		Symbol:   1,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		Price:    100,
		Quantity: 5,
	}

	ob.AddOrder(ss, order1)
	ob.AddOrder(ss, order2)

	level := ss.BidIndex[100]
	if level.Quantity != 15 {
		t.Errorf("expected quantity 15, got %d", level.Quantity)
	}
	if level.Orders.Len() != 2 {
		t.Errorf("expected 2 orders, got %d", level.Orders.Len())
	}
}

func TestAddOrderNewPriceLevel(t *testing.T) {
	ob := New()
	ss := &state.OrderBookState{
		BidIndex: make(map[types.Price]*state.PriceLevel),
		AskIndex: make(map[types.Price]*state.PriceLevel),
	}

	order1 := &types.Order{
		ID:       1,
		Side:     constants.ORDER_SIDE_BUY,
		Price:    100,
		Quantity: 10,
	}
	order2 := &types.Order{
		ID:       2,
		Side:     constants.ORDER_SIDE_BUY,
		Price:    101,
		Quantity: 5,
	}

	ob.AddOrder(ss, order1)
	ob.AddOrder(ss, order2)

	if ss.BidIndex[100] == nil {
		t.Error("expected BidIndex[100] to exist")
	}
	if ss.BidIndex[101] == nil {
		t.Error("expected BidIndex[101] to exist")
	}
	if ss.BestBid != ss.BidIndex[101] {
		t.Errorf("expected BestBid to be 101 level, got %v", ss.BestBid)
	}
	if ss.BestBid.NextBid != ss.BidIndex[100] {
		t.Error("expected NextBid to point to 100 level")
	}
}

func TestRemoveOrder(t *testing.T) {
	ob := New()
	ss := &state.OrderBookState{
		BidIndex: make(map[types.Price]*state.PriceLevel),
		AskIndex: make(map[types.Price]*state.PriceLevel),
	}

	order := &types.Order{
		ID:       1,
		UserID:   1,
		Symbol:   1,
		Side:     constants.ORDER_SIDE_BUY,
		Price:    100,
		Quantity: 10,
		Filled:   0,
	}

	ob.AddOrder(ss, order)
	ob.RemoveOrder(ss, order)

	if _, ok := ss.BidIndex[100]; ok {
		t.Error("expected BidIndex[100] to be nil after removing only order")
	}
	if ss.BestBid != nil {
		t.Error("expected BestBid to be nil")
	}
}

func TestRemoveOrderPartial(t *testing.T) {
	ob := New()
	ss := &state.OrderBookState{
		BidIndex: make(map[types.Price]*state.PriceLevel),
		AskIndex: make(map[types.Price]*state.PriceLevel),
	}

	order := &types.Order{
		ID:       1,
		UserID:   1,
		Symbol:   1,
		Side:     constants.ORDER_SIDE_BUY,
		Price:    100,
		Quantity: 10,
		Filled:   5,
	}

	ob.AddOrder(ss, order)

	level := ss.BidIndex[100]
	if level == nil {
		t.Fatal("expected BidIndex[100] to exist after AddOrder")
	}
	if level.Quantity != 5 {
		t.Errorf("expected quantity 5 after AddOrder, got %d", level.Quantity)
	}

	ob.RemoveOrder(ss, order)

	if _, ok := ss.BidIndex[100]; ok {
		t.Error("expected BidIndex[100] to be nil after removing all remaining quantity")
	}
}

func TestGetBestBidAsk(t *testing.T) {
	ob := New()
	ss := &state.OrderBookState{
		BidIndex: make(map[types.Price]*state.PriceLevel),
		AskIndex: make(map[types.Price]*state.PriceLevel),
	}

	ss.BidIndex[99] = &state.PriceLevel{Price: 99}
	ss.BestBid = ss.BidIndex[99]

	ss.AskIndex[101] = &state.PriceLevel{Price: 101}
	ss.BestAsk = ss.AskIndex[101]

	if ob.GetBestBid(ss) != 99 {
		t.Errorf("expected best bid 99, got %d", ob.GetBestBid(ss))
	}
	if ob.GetBestAsk(ss) != 101 {
		t.Errorf("expected best ask 101, got %d", ob.GetBestAsk(ss))
	}
}

func TestGetDepth(t *testing.T) {
	ob := New()
	ss := &state.OrderBookState{
		BidIndex: make(map[types.Price]*state.PriceLevel),
		AskIndex: make(map[types.Price]*state.PriceLevel),
	}

	level100 := &state.PriceLevel{Price: 100, Quantity: 10}
	level101 := &state.PriceLevel{Price: 101, Quantity: 5}

	ss.BidIndex[100] = level100
	ss.BidIndex[101] = level101
	ss.BestBid = level101
	level101.NextBid = level100

	depth := ob.GetDepth(ss, constants.ORDER_SIDE_BUY, 10)
	if len(depth) != 4 {
		t.Errorf("expected 4 values (2 levels * 2), got %d", len(depth))
	}
	if depth[0] != 101 {
		t.Errorf("expected first price at 101, got %d", depth[0])
	}
	if depth[2] != 100 {
		t.Errorf("expected second price at 100, got %d", depth[2])
	}

	depth = ob.GetDepth(ss, constants.ORDER_SIDE_BUY, 1)
	if len(depth) != 2 {
		t.Errorf("expected 2 values with limit, got %d", len(depth))
	}
}

func TestRemoveOrderWithMultipleOrders(t *testing.T) {
	ob := New()
	ss := &state.OrderBookState{
		BidIndex: make(map[types.Price]*state.PriceLevel),
		AskIndex: make(map[types.Price]*state.PriceLevel),
	}

	order1 := &types.Order{ID: 1, Side: constants.ORDER_SIDE_BUY, Price: 100, Quantity: 5, Filled: 0}
	order2 := &types.Order{ID: 2, Side: constants.ORDER_SIDE_BUY, Price: 100, Quantity: 5, Filled: 0}

	ob.AddOrder(ss, order1)
	ob.AddOrder(ss, order2)

	level := ss.BidIndex[100]
	if level.Orders.Len() != 2 {
		t.Errorf("expected 2 orders before removal, got %d", level.Orders.Len())
	}

	ob.RemoveOrder(ss, order1)

	level = ss.BidIndex[100]
	if level == nil {
		t.Fatal("expected BidIndex[100] to still exist")
	}
	if level.Orders.Len() != 1 {
		t.Errorf("expected 1 order after removal, got %d", level.Orders.Len())
	}
	if level.Quantity != 5 {
		t.Errorf("expected quantity 5, got %d", level.Quantity)
	}
}

func TestEmptyOrderBook(t *testing.T) {
	ob := New()
	ss := &state.OrderBookState{
		BidIndex: make(map[types.Price]*state.PriceLevel),
		AskIndex: make(map[types.Price]*state.PriceLevel),
	}

	if ob.GetBestBid(ss) != 0 {
		t.Error("expected best bid 0 for empty book")
	}
	if ob.GetBestAsk(ss) != 0 {
		t.Error("expected best ask 0 for empty book")
	}
}
