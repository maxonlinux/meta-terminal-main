package oms

import (
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func BenchmarkValidateOrder(b *testing.B) {
	b.ReportAllocs()
	s, _ := newTestService()
	input := &types.OrderInput{
		UserID:   1,
		Symbol:   "BTCUSDT",
		Category: constants.CATEGORY_LINEAR,
		Side:     constants.ORDER_SIDE_BUY,
		Type:     constants.ORDER_TYPE_LIMIT,
		TIF:      constants.TIF_GTC,
		Quantity: 1,
		Price:    100,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = s.validateOrder(input)
	}
}
