package pool

import (
	"testing"

	"github.com/anomalyco/meta-terminal-go/types"
)

func BenchmarkGetOrder(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		order := GetOrder()
		order.ID = types.OrderID(i)
		order.UserID = 1
		PutOrder(order)
	}
}

func BenchmarkGetTrade(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		trade := GetTrade()
		trade.ID = types.TradeID(i)
		PutTrade(trade)
	}
}

func BenchmarkGetOrderResult(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		result := GetOrderResult()
		PutOrderResult(result)
	}
}

func BenchmarkNextOrderID(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		NextOrderID()
	}
}
