package oms

import (
	"testing"

	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/robaho/fixed"
)

func TestReduceOnlyIndex_Add(t *testing.T) {
	r := NewReduceOnlyIndex()

	order := &types.Order{
		ID:         types.OrderID(1),
		UserID:     types.UserID(1),
		Symbol:     "BTCUSDT",
		Side:       constants.ORDER_SIDE_BUY,
		Quantity:   types.Quantity(fixed.NewI(10, 0)),
		Filled:     types.Quantity(fixed.NewI(0, 0)),
		ReduceOnly: true,
	}

	r.Add(order)

	// Check exposure via internal access
	shardIdx := ShardIndex("BTCUSDT")
	shard := r.shards[shardIdx]
	if shard.exposure[types.UserID(1)].Cmp(types.Quantity(fixed.NewI(10, 0))) != 0 {
		t.Errorf("expected exposure 10, got %d", shard.exposure[types.UserID(1)])
	}
}

func TestReduceOnlyIndex_AddNonReduceOnly(t *testing.T) {
	r := NewReduceOnlyIndex()

	order := &types.Order{
		ID:         types.OrderID(1),
		UserID:     types.UserID(1),
		Symbol:     "BTCUSDT",
		Side:       constants.ORDER_SIDE_BUY,
		Quantity:   types.Quantity(fixed.NewI(10, 0)),
		Filled:     types.Quantity(fixed.NewI(0, 0)),
		ReduceOnly: false,
	}

	r.Add(order)

	// Non-reduce-only should not add exposure
	shardIdx := ShardIndex("BTCUSDT")
	shard := r.shards[shardIdx]
	if shard.exposure[types.UserID(1)].Sign() != 0 {
		t.Error("non-reduce-only order should not add exposure")
	}
}

func TestReduceOnlyIndex_Remove(t *testing.T) {
	r := NewReduceOnlyIndex()

	order := &types.Order{
		ID:         types.OrderID(1),
		UserID:     types.UserID(1),
		Symbol:     "BTCUSDT",
		Side:       constants.ORDER_SIDE_BUY,
		Quantity:   types.Quantity(fixed.NewI(10, 0)),
		Filled:     types.Quantity(fixed.NewI(0, 0)),
		ReduceOnly: true,
	}

	r.Add(order)
	r.Remove(order)

	shardIdx := ShardIndex("BTCUSDT")
	shard := r.shards[shardIdx]
	if shard.exposure[types.UserID(1)].Cmp(types.Quantity(fixed.NewI(0, 0))) != 0 {
		t.Errorf("expected exposure 0, got %d", shard.exposure[types.UserID(1)])
	}
}

func TestReduceOnlyIndex_OnPositionReduce(t *testing.T) {
	r := NewReduceOnlyIndex()

	order1 := &types.Order{
		ID:         types.OrderID(1),
		UserID:     types.UserID(1),
		Symbol:     "BTCUSDT",
		Side:       constants.ORDER_SIDE_SELL,
		Quantity:   types.Quantity(fixed.NewI(10, 0)),
		Filled:     types.Quantity(fixed.NewI(0, 0)),
		ReduceOnly: true,
		Price:      types.Price(fixed.NewI(51000, 0)),
	}

	order2 := &types.Order{
		ID:         types.OrderID(2),
		UserID:     types.UserID(1),
		Symbol:     "BTCUSDT",
		Side:       constants.ORDER_SIDE_SELL,
		Quantity:   types.Quantity(fixed.NewI(10, 0)),
		Filled:     types.Quantity(fixed.NewI(0, 0)),
		ReduceOnly: true,
		Price:      types.Price(fixed.NewI(52000, 0)),
	}

	r.Add(order1)
	r.Add(order2)

	service := NewService()
	service.reduceonly = r
	service.OnPositionReduce(types.UserID(1), "BTCUSDT", types.Quantity(fixed.NewI(5, 0)))

	if order1.Status != constants.ORDER_STATUS_CANCELED {
		t.Errorf("order1 should be fully canceled, got status %d", order1.Status)
	}
}

func TestReduceOnlyIndex_OnPositionReduce_MissingSymbol(t *testing.T) {
	r := NewReduceOnlyIndex()

	// Ensure missing shard maps do not panic for short positions.
	r.OnPositionReduce("BTCUSDT", types.Quantity(fixed.NewI(-5, 0)), types.UserID(1))
}

func TestReduceOnlyIndex_ShardDistribution(t *testing.T) {
	r := NewReduceOnlyIndex()

	// Add orders with 1000 different symbols
	for i := 0; i < 1000; i++ {
		symbol := string(rune('A'+i%26)) + string(rune('0'+i/26))
		order := &types.Order{
			ID:         types.OrderID(i),
			UserID:     types.UserID(i % 100),
			Symbol:     symbol,
			Side:       constants.ORDER_SIDE_BUY,
			Quantity:   types.Quantity(fixed.NewI(10, 0)),
			Filled:     types.Quantity(fixed.NewI(0, 0)),
			ReduceOnly: true,
		}
		r.Add(order)
	}

	// Verify exposure is stored in correct shards by checking each shard's heaps
	shardCounts := make(map[uint8]int)
	for shardIdx, symbolMap := range r.buyHeaps {
		count := 0
		for range symbolMap {
			count++
		}
		shardCounts[shardIdx] = count
	}
	for shardIdx, symbolMap := range r.sellHeaps {
		count := shardCounts[shardIdx]
		for range symbolMap {
			count++
		}
		shardCounts[shardIdx] = count
	}

	// Count total tracked symbols and per-shard distribution
	totalTracked := 0
	for shardIdx, count := range shardCounts {
		totalTracked += count
		if count > 10 {
			t.Errorf("shard %d has too many symbols: %d", shardIdx, count)
		}
	}

	// Verify total tracked symbols
	if totalTracked != 1000 {
		t.Errorf("expected 1000 symbols tracked across shards, got %d", totalTracked)
	}
}

func BenchmarkReduceOnlyIndex_Add(b *testing.B) {
	r := NewReduceOnlyIndex()

	order := &types.Order{
		ID:         types.OrderID(0),
		UserID:     types.UserID(1),
		Symbol:     "BTCUSDT",
		Side:       constants.ORDER_SIDE_BUY,
		Quantity:   types.Quantity(fixed.NewI(10, 0)),
		Filled:     types.Quantity(fixed.NewI(0, 0)),
		ReduceOnly: true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		order.ID = types.OrderID(i)
		r.Add(order)
	}
}

func BenchmarkReduceOnlyIndex_Add1000Symbols(b *testing.B) {
	r := NewReduceOnlyIndex()

	orders := make([]*types.Order, 1000)
	for i := 0; i < 1000; i++ {
		orders[i] = &types.Order{
			ID:         types.OrderID(i),
			UserID:     types.UserID(i % 100),
			Symbol:     string(rune('A'+i%26)) + string(rune('0'+i/26)),
			Side:       constants.ORDER_SIDE_BUY,
			Quantity:   types.Quantity(fixed.NewI(10, 0)),
			Filled:     types.Quantity(fixed.NewI(0, 0)),
			ReduceOnly: true,
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 1000; j++ {
			r.Add(orders[j])
		}
	}
}

func BenchmarkReduceOnlyIndex_OnPositionReduce(b *testing.B) {
	r := NewReduceOnlyIndex()

	for i := 0; i < 1000; i++ {
		order := &types.Order{
			ID:         types.OrderID(i),
			UserID:     types.UserID(1),
			Symbol:     "BTCUSDT",
			Side:       constants.ORDER_SIDE_SELL,
			Quantity:   types.Quantity(fixed.NewI(10, 0)),
			Filled:     types.Quantity(fixed.NewI(0, 0)),
			ReduceOnly: true,
			Price:      types.Price(fixed.NewI(int64(50000+i), 0)),
		}
		r.Add(order)
	}

	service := NewService()
	service.reduceonly = r

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.OnPositionReduce(types.UserID(1), "BTCUSDT", types.Quantity(fixed.NewI(5000, 0)))
	}
}

func BenchmarkReduceOnlyIndex_OnPositionReduce1000Symbols(b *testing.B) {
	r := NewReduceOnlyIndex()

	// Add orders across 1000 symbols
	for i := 0; i < 1000; i++ {
		symbol := string(rune('A'+i%26)) + string(rune('0'+i/26))
		order := &types.Order{
			ID:         types.OrderID(i),
			UserID:     types.UserID(1),
			Symbol:     symbol,
			Side:       constants.ORDER_SIDE_SELL,
			Quantity:   types.Quantity(fixed.NewI(10, 0)),
			Filled:     types.Quantity(fixed.NewI(0, 0)),
			ReduceOnly: true,
			Price:      types.Price(fixed.NewI(int64(50000+i), 0)),
		}
		r.Add(order)
	}

	service := NewService()
	service.reduceonly = r

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Reduce position for each symbol
		for j := 0; j < 1000; j++ {
			symbol := string(rune('A'+j%26)) + string(rune('0'+j/26))
			service.OnPositionReduce(types.UserID(1), symbol, types.Quantity(fixed.NewI(5, 0)))
		}
	}
}
