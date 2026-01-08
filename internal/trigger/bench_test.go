package trigger

import (
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func BenchmarkCheck(b *testing.B) {
	mon := NewMonitor()
	b.ReportAllocs()
	for i := range 1000 {
		mon.Add(types.OrderID(i+1), constants.ORDER_SIDE_BUY, types.Price(10000+i))
	}

	for b.Loop() {
		_ = mon.Check(9500)
	}
}
