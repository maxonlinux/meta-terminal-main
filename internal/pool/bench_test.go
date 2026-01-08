package pool

import (
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func BenchmarkOrderPoolGetPut(b *testing.B) {
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		order := GetOrder()
		order.ID = types.OrderID(i)
		order.UserID = types.UserID(i % 100)
		order.Symbol = "BTCUSDT"
		order.Price = 50000
		order.Quantity = 10
		PutOrder(order)
	}
}

func BenchmarkTradePoolGetPut(b *testing.B) {
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		trade := GetTrade()
		trade.ID = types.TradeID(i)
		trade.Symbol = "BTCUSDT"
		trade.TakerID = 1
		trade.MakerID = 2
		trade.Price = 50000
		trade.Quantity = 10
		PutTrade(trade)
	}
}

func BenchmarkOrderResultPoolGetPut(b *testing.B) {
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		res := GetOrderResult()
		res.Order = &types.Order{ID: types.OrderID(i)}
		PutOrderResult(res)
	}
}
