package positions

import (
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
)

func BenchmarkPositionUpdate(b *testing.B) {
	pos := New("BTCUSDT")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pos.Update(1, 50000, constants.SIDE_LONG, 10)
	}
}
