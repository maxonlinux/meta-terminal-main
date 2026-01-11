package orderbook

import (
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/pool"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func TestOrderBook_AddRemove(t *testing.T) {
	ob := New()

	order := pool.GetOrder()
	order.ID = types.OrderID(1)
	order.UserID = types.UserID(1)
	order.Symbol = "BTCUSDT"
	order.Category = constants.CATEGORY_SPOT
	order.Side = constants.ORDER_SIDE_BUY
	order.Type = constants.ORDER_TYPE_LIMIT
	order.Price = types.Price(50000)
	order.Quantity = types.Quantity(10)
	order.Filled = 0
	order.CreatedAt = types.NowNano()

	ob.Add(order)

	bid, qty, ok := ob.BestBid()
	if !ok {
		t.Fatal("expected best bid")
	}
	if bid != 50000 {
		t.Errorf("expected price 50000, got %d", bid)
	}
	if qty != 10 {
		t.Errorf("expected qty 10, got %d", qty)
	}

	if !ob.Remove(order.ID) {
		t.Fatal("expected Remove to return true")
	}

	_, _, ok = ob.BestBid()
	if ok {
		t.Fatal("expected no best bid after remove")
	}
}

func TestOrderBook_Match(t *testing.T) {
	ob := New()

	seller := pool.GetOrder()
	seller.ID = types.OrderID(1)
	seller.UserID = types.UserID(2)
	seller.Symbol = "BTCUSDT"
	seller.Category = constants.CATEGORY_SPOT
	seller.Side = constants.ORDER_SIDE_SELL
	seller.Type = constants.ORDER_TYPE_LIMIT
	seller.Price = types.Price(50000)
	seller.Quantity = types.Quantity(10)
	seller.Filled = 0
	seller.CreatedAt = types.NowNano()
	ob.Add(seller)

	buyer := pool.GetOrder()
	buyer.ID = types.OrderID(2)
	buyer.UserID = types.UserID(1)
	buyer.Symbol = "BTCUSDT"
	buyer.Category = constants.CATEGORY_SPOT
	buyer.Side = constants.ORDER_SIDE_BUY
	buyer.Type = constants.ORDER_TYPE_LIMIT
	buyer.Price = types.Price(50000)
	buyer.Quantity = types.Quantity(5)
	buyer.Filled = 0
	buyer.CreatedAt = types.NowNano()

	trades, err := ob.Match(buyer, buyer.Price)
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}

	if len(trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(trades))
	}
	if trades[0].Trade.Quantity != 5 {
		t.Errorf("expected trade qty 5, got %d", trades[0].Trade.Quantity)
	}
	if buyer.Filled != 5 {
		t.Errorf("expected buyer filled 5, got %d", buyer.Filled)
	}
	if seller.Filled != 5 {
		t.Errorf("expected seller filled 5, got %d", seller.Filled)
	}
}

func TestOrderBook_Depth(t *testing.T) {
	ob := New()

	prices := []types.Price{50000, 49900, 49800, 49700, 49600}
	for i, price := range prices {
		order := pool.GetOrder()
		order.ID = types.OrderID(uint64(i + 1))
		order.UserID = types.UserID(1)
		order.Symbol = "BTCUSDT"
		order.Category = constants.CATEGORY_SPOT
		order.Side = constants.ORDER_SIDE_BUY
		order.Type = constants.ORDER_TYPE_LIMIT
		order.Price = price
		order.Quantity = types.Quantity(10 * (i + 1))
		order.Filled = 0
		order.CreatedAt = types.NowNano()
		ob.Add(order)
	}

	bidPrices, _, askPrices, _ := ob.Depth(3)

	if len(bidPrices) != 3 {
		t.Errorf("expected 3 bid prices, got %d", len(bidPrices))
	}
	if len(askPrices) != 0 {
		t.Errorf("expected 0 ask prices, got %d", len(askPrices))
	}
	if bidPrices[0] != 50000 {
		t.Errorf("expected best bid 50000, got %d", bidPrices[0])
	}
}

func TestOrderBook_WouldCross(t *testing.T) {
	ob := New()

	askOrder := pool.GetOrder()
	askOrder.ID = types.OrderID(1)
	askOrder.UserID = types.UserID(2)
	askOrder.Symbol = "BTCUSDT"
	askOrder.Category = constants.CATEGORY_SPOT
	askOrder.Side = constants.ORDER_SIDE_SELL
	askOrder.Type = constants.ORDER_TYPE_LIMIT
	askOrder.Price = types.Price(50000)
	askOrder.Quantity = types.Quantity(10)
	askOrder.Filled = 0
	askOrder.CreatedAt = types.NowNano()
	ob.Add(askOrder)

	if ob.WouldCross(constants.ORDER_SIDE_BUY, 49999) {
		t.Error("buy at 49999 should not cross ask at 50000")
	}

	if !ob.WouldCross(constants.ORDER_SIDE_BUY, 50000) {
		t.Error("buy at 50000 should cross ask at 50000")
	}
}

func BenchmarkOrderBook_Match(b *testing.B) {
	b.ReportAllocs()
	ob := New()

	for i := 0; i < 1000; i++ {
		order := pool.GetOrder()
		order.ID = types.OrderID(uint64(i + 1))
		order.UserID = types.UserID(1)
		order.Symbol = "BTCUSDT"
		order.Category = constants.CATEGORY_SPOT
		order.Side = constants.ORDER_SIDE_SELL
		order.Type = constants.ORDER_TYPE_LIMIT
		order.Price = types.Price(50000 + i%100)
		order.Quantity = types.Quantity(10)
		order.Filled = 0
		order.CreatedAt = types.NowNano()
		ob.Add(order)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buyer := pool.GetOrder()
		buyer.ID = types.OrderID(uint64(10000 + i))
		buyer.UserID = types.UserID(2)
		buyer.Symbol = "BTCUSDT"
		buyer.Category = constants.CATEGORY_SPOT
		buyer.Side = constants.ORDER_SIDE_BUY
		buyer.Type = constants.ORDER_TYPE_LIMIT
		buyer.Price = types.Price(50100)
		buyer.Quantity = types.Quantity(10)
		buyer.Filled = 0
		buyer.CreatedAt = types.NowNano()
		_, _ = ob.Match(buyer, buyer.Price)
	}
}
