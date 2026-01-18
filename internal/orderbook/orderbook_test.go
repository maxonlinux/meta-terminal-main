package orderbook

import (
	"testing"

	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/robaho/fixed"
)

const scl = 8

func p10(n int) int64 {
	r := int64(1)
	for i := 0; i < n; i++ {
		r *= 10
	}
	return r
}

func p(v int64) types.Price {
	return types.Price(fixed.NewI(v*p10(scl), scl))
}

func q(v int64) types.Quantity {
	return types.Quantity(fixed.NewI(v*p10(scl), scl))
}

func TestAdd(t *testing.T) {
	ob := New()
	o := &types.Order{
		ID:       types.OrderID(1),
		UserID:   types.UserID(1),
		Symbol:   "BTC-USD",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Price:    p(50000),
		Quantity: q(10),
	}
	ob.Add(o)

	available := ob.AvailableQuantity(constants.ORDER_SIDE_SELL, types.Price{}, q(1))
	if available.Sign() <= 0 {
		t.Fatal("expected available liquidity")
	}
}

func TestRemove(t *testing.T) {
	ob := New()
	o := &types.Order{
		ID:       types.OrderID(1),
		UserID:   types.UserID(1),
		Symbol:   "BTC-USD",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Price:    p(50000),
		Quantity: q(10),
	}
	ob.Add(o)

	if !ob.Remove(o.ID) {
		t.Fatal("remove failed")
	}

	available := ob.AvailableQuantity(constants.ORDER_SIDE_SELL, types.Price{}, q(1))
	if available.Sign() > 0 {
		t.Fatal("expected empty book")
	}
}

func TestMatch(t *testing.T) {
	ob := New()
	sell := &types.Order{
		ID:       types.OrderID(1),
		UserID:   types.UserID(2),
		Symbol:   "BTC-USD",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_SELL,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Price:    p(50000),
		Quantity: q(10),
	}
	ob.Add(sell)
	buy := &types.Order{
		ID:       types.OrderID(2),
		UserID:   types.UserID(1),
		Symbol:   "BTC-USD",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Price:    p(50000),
		Quantity: q(5),
	}

	var trades []types.Match
	ob.Match(buy, types.Price{}, func(match types.Match) {
		trades = append(trades, match)
	})

	if len(trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(trades))
	}
	if trades[0].Quantity.Cmp(q(5)) != 0 {
		t.Fatalf("expected qty 5, got %s", trades[0].Quantity)
	}
	if buy.Filled.Cmp(q(5)) != 0 {
		t.Fatalf("expected filled 5, got %s", buy.Filled)
	}
	if sell.Filled.Cmp(q(5)) != 0 {
		t.Fatalf("expected seller filled 5, got %s", sell.Filled)
	}
}

func BenchmarkAdd(b *testing.B) {
	ob := New()
	var orderID types.OrderID
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		orderID++
		ob.Add(&types.Order{
			ID:       orderID,
			UserID:   types.UserID(1),
			Symbol:   "BTC-USD",
			Category: constants.CATEGORY_LINEAR,
			Side:     constants.ORDER_SIDE_BUY,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Price:    p(50000 + int64(i%1000)),
			Quantity: q(1),
		})
	}
}

func BenchmarkAddOnce(b *testing.B) {
	ob := New()
	var orderID types.OrderID
	for j := 0; j < 1000; j++ {
		orderID++
		ob.Add(&types.Order{
			ID:       orderID,
			UserID:   types.UserID(1),
			Symbol:   "BTC-USD",
			Category: constants.CATEGORY_LINEAR,
			Side:     constants.ORDER_SIDE_BUY,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Price:    p(50000 + int64(j)),
			Quantity: q(10),
		})
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ob2 := New()
		var ordID types.OrderID
		for j := 0; j < 1000; j++ {
			ordID++
			ob2.Add(&types.Order{
				ID:       ordID,
				UserID:   types.UserID(1),
				Symbol:   "BTC-USD",
				Category: constants.CATEGORY_LINEAR,
				Side:     constants.ORDER_SIDE_BUY,
				Type:     constants.ORDER_TYPE_LIMIT,
				TIF:      constants.TIF_GTC,
				Price:    p(50000 + int64(j)),
				Quantity: q(10),
			})
		}
	}
}

func BenchmarkAddReused(b *testing.B) {
	ob := New()
	var orderID types.OrderID
	for j := 0; j < 1000; j++ {
		orderID++
		ob.Add(&types.Order{
			ID:       orderID,
			UserID:   types.UserID(1),
			Symbol:   "BTC-USD",
			Category: constants.CATEGORY_LINEAR,
			Side:     constants.ORDER_SIDE_BUY,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Price:    p(50000 + int64(j)),
			Quantity: q(10),
		})
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 1000; j++ {
			ob.Remove(types.OrderID(j + 1))
		}
		for j := 0; j < 1000; j++ {
			ob.Add(&types.Order{
				ID:       types.OrderID(j + 1),
				UserID:   types.UserID(1),
				Symbol:   "BTC-USD",
				Category: constants.CATEGORY_LINEAR,
				Side:     constants.ORDER_SIDE_BUY,
				Type:     constants.ORDER_TYPE_LIMIT,
				TIF:      constants.TIF_GTC,
				Price:    p(50000 + int64(j)),
				Quantity: q(10),
			})
		}
	}
}

func BenchmarkAddAllDifferentLevels(b *testing.B) {
	ob := New()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 1000; j++ {
			ob.Add(&types.Order{
				ID:       types.OrderID(i*1000 + j),
				UserID:   types.UserID(1),
				Symbol:   "BTC-USD",
				Category: constants.CATEGORY_LINEAR,
				Side:     constants.ORDER_SIDE_BUY,
				Type:     constants.ORDER_TYPE_LIMIT,
				TIF:      constants.TIF_GTC,
				Price:    p(50000 + int64(j)),
				Quantity: q(1),
			})
		}
	}
}

func BenchmarkAddAllSameLevel(b *testing.B) {
	ob := New()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 1000; j++ {
			ob.Add(&types.Order{
				ID:       types.OrderID(i*1000 + j),
				UserID:   types.UserID(1),
				Symbol:   "BTC-USD",
				Category: constants.CATEGORY_LINEAR,
				Side:     constants.ORDER_SIDE_BUY,
				Type:     constants.ORDER_TYPE_LIMIT,
				TIF:      constants.TIF_GTC,
				Price:    p(50000),
				Quantity: q(1),
			})
		}
	}
}

func BenchmarkMatchAllDifferentLevels(b *testing.B) {
	ob := New()
	for j := 0; j < 1000; j++ {
		ob.Add(&types.Order{
			ID:       types.OrderID(j),
			UserID:   types.UserID(2),
			Symbol:   "BTC-USD",
			Category: constants.CATEGORY_LINEAR,
			Side:     constants.ORDER_SIDE_SELL,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Price:    p(50000 + int64(j)),
			Quantity: q(1),
		})
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		taker := &types.Order{
			ID:       types.OrderID(i),
			UserID:   types.UserID(1),
			Symbol:   "BTC-USD",
			Category: constants.CATEGORY_LINEAR,
			Side:     constants.ORDER_SIDE_BUY,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Price:    p(60000),
			Quantity: q(1000),
		}
		ob.Match(taker, types.Price{}, func(match types.Match) {})
	}
}

func BenchmarkMatchAllSameLevel(b *testing.B) {
	ob := New()
	for j := 0; j < 1000; j++ {
		ob.Add(&types.Order{
			ID:       types.OrderID(j),
			UserID:   types.UserID(2),
			Symbol:   "BTC-USD",
			Category: constants.CATEGORY_LINEAR,
			Side:     constants.ORDER_SIDE_SELL,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Price:    p(50000),
			Quantity: q(1),
		})
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		taker := &types.Order{
			ID:       types.OrderID(i),
			UserID:   types.UserID(1),
			Symbol:   "BTC-USD",
			Category: constants.CATEGORY_LINEAR,
			Side:     constants.ORDER_SIDE_BUY,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Price:    p(50000),
			Quantity: q(1000),
		}
		ob.Match(taker, types.Price{}, func(match types.Match) {})
	}
}

func BenchmarkMatchManyTrades(b *testing.B) {
	ob := New()
	for j := 0; j < 10000; j++ {
		ob.Add(&types.Order{
			ID:       types.OrderID(j),
			UserID:   types.UserID(2),
			Symbol:   "BTC-USD",
			Category: constants.CATEGORY_LINEAR,
			Side:     constants.ORDER_SIDE_SELL,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Price:    p(50000),
			Quantity: q(1),
		})
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		taker := &types.Order{
			ID:       types.OrderID(i),
			UserID:   types.UserID(1),
			Symbol:   "BTC-USD",
			Category: constants.CATEGORY_LINEAR,
			Side:     constants.ORDER_SIDE_BUY,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Price:    p(50000),
			Quantity: q(10000),
		}
		ob.Match(taker, types.Price{}, func(match types.Match) {})
	}
}

func BenchmarkAddManyLevels(b *testing.B) {
	ob := New()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 10000; j++ {
			ob.Add(&types.Order{
				ID:       types.OrderID(i*10000 + j),
				UserID:   types.UserID(1),
				Symbol:   "BTC-USD",
				Category: constants.CATEGORY_LINEAR,
				Side:     constants.ORDER_SIDE_BUY,
				Type:     constants.ORDER_TYPE_LIMIT,
				TIF:      constants.TIF_GTC,
				Price:    p(50000 + int64(j)),
				Quantity: q(1),
			})
		}
	}
}

func BenchmarkMatchLevelTraversal(b *testing.B) {
	ob := New()
	for j := 0; j < 10000; j++ {
		ob.Add(&types.Order{
			ID:       types.OrderID(j),
			UserID:   types.UserID(2),
			Symbol:   "BTC-USD",
			Category: constants.CATEGORY_LINEAR,
			Side:     constants.ORDER_SIDE_SELL,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Price:    p(50000 + int64(j)),
			Quantity: q(1),
		})
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		taker := &types.Order{
			ID:       types.OrderID(i),
			UserID:   types.UserID(1),
			Symbol:   "BTC-USD",
			Category: constants.CATEGORY_LINEAR,
			Side:     constants.ORDER_SIDE_BUY,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Price:    p(60000),
			Quantity: q(10000),
		}
		ob.Match(taker, types.Price{}, func(match types.Match) {})
	}
}

func BenchmarkMixedWorkload(b *testing.B) {
	ob := New()
	for j := 0; j < 10000; j++ {
		ob.Add(&types.Order{
			ID:       types.OrderID(j),
			UserID:   types.UserID(2),
			Symbol:   "BTC-USD",
			Category: constants.CATEGORY_LINEAR,
			Side:     constants.ORDER_SIDE_SELL,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Price:    p(50000 + int64(j%1000)),
			Quantity: q(1),
		})
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 1000; j++ {
			taker := &types.Order{
				ID:       types.OrderID(i*1000 + j),
				UserID:   types.UserID(1),
				Symbol:   "BTC-USD",
				Category: constants.CATEGORY_LINEAR,
				Side:     constants.ORDER_SIDE_BUY,
				Type:     constants.ORDER_TYPE_LIMIT,
				TIF:      constants.TIF_GTC,
				Price:    p(50500),
				Quantity: q(10),
			}
			ob.Match(taker, types.Price{}, func(match types.Match) {})
		}
	}
}

func BenchmarkMatch(b *testing.B) {
	ob := New()
	var orderID types.OrderID

	for j := 0; j < 1000; j++ {
		orderID++
		ob.Add(&types.Order{
			ID:       orderID,
			UserID:   types.UserID(2),
			Symbol:   "BTC-USD",
			Category: constants.CATEGORY_LINEAR,
			Side:     constants.ORDER_SIDE_SELL,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Price:    p(50000 + int64(j)),
			Quantity: q(10),
		})
	}

	var takerID types.OrderID
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		takerID++
		taker := &types.Order{
			ID:       takerID,
			UserID:   types.UserID(1),
			Symbol:   "BTC-USD",
			Category: constants.CATEGORY_LINEAR,
			Side:     constants.ORDER_SIDE_BUY,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Price:    p(50500),
			Quantity: q(1),
		}
		ob.Match(taker, types.Price{}, func(match types.Match) {})
	}
}

func BenchmarkRemove(b *testing.B) {
	ob := New()
	var orderID types.OrderID
	var ids []types.OrderID
	for j := 0; j < 1000; j++ {
		orderID++
		ids = append(ids, orderID)
		ob.Add(&types.Order{
			ID:       orderID,
			UserID:   types.UserID(1),
			Symbol:   "BTC-USD",
			Category: constants.CATEGORY_LINEAR,
			Side:     constants.ORDER_SIDE_BUY,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Price:    p(50000 + int64(j%1000)),
			Quantity: q(10),
		})
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ob.Remove(ids[i%1000])
	}
}

func BenchmarkMatchBatch(b *testing.B) {
	ob := New()
	var orderID types.OrderID

	for j := 0; j < 1000; j++ {
		orderID++
		ob.Add(&types.Order{
			ID:       orderID,
			UserID:   types.UserID(2),
			Symbol:   "BTC-USD",
			Category: constants.CATEGORY_LINEAR,
			Side:     constants.ORDER_SIDE_SELL,
			Type:     constants.ORDER_TYPE_LIMIT,
			TIF:      constants.TIF_GTC,
			Price:    p(50000),
			Quantity: q(10),
		})
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 1000; j++ {
			taker := &types.Order{
				ID:       types.OrderID(i*1000 + j),
				UserID:   types.UserID(1),
				Symbol:   "BTC-USD",
				Category: constants.CATEGORY_LINEAR,
				Side:     constants.ORDER_SIDE_BUY,
				Type:     constants.ORDER_TYPE_LIMIT,
				TIF:      constants.TIF_GTC,
				Price:    p(50000),
				Quantity: q(1),
			}
			ob.Match(taker, types.Price{}, func(match types.Match) {})
		}
	}
}
