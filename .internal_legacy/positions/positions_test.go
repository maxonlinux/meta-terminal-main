package positions

import (
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/domain"
)

func TestPositionUpdateAndLiquidation(t *testing.T) {
	p := New("BTCUSDT")
	p.Update(10, 1000, constants.SIDE_LONG, 10)

	if p.Size != 10 {
		t.Fatalf("size=%d", p.Size)
	}
	if p.EntryPrice != 1000 {
		t.Fatalf("entry=%d", p.EntryPrice)
	}
	if p.LiquidationPrice == 0 {
		t.Fatal("expected liquidation price")
	}
	if p.ShouldLiquidate(domain.Price(0)) {
		t.Fatal("should not liquidate at 0 (invalid price)")
	}
}
