package position

import (
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/types"
)

func BenchmarkUpdatePosition(b *testing.B) {
	s := state.NewEngineState()
	userID := types.UserID(1)
	symbol := "BTCUSDT"

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		UpdatePosition(s, userID, symbol, types.Quantity(1), types.Price(50000), 0, 10)
	}
}

func BenchmarkCalculatePositionRisk(b *testing.B) {
	pos := &types.Position{
		Size:       types.Quantity(10),
		Side:       0,
		EntryPrice: types.Price(50000),
		Leverage:   10,
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		CalculatePositionRisk(pos)
	}
}

func BenchmarkGetLeverage(b *testing.B) {
	s := state.NewEngineState()
	userID := types.UserID(1)
	symbol := "BTCUSDT"
	UpdatePosition(s, userID, symbol, types.Quantity(1), types.Price(50000), 0, 10)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		GetLeverage(s, userID, symbol)
	}
}

func BenchmarkGetPosition(b *testing.B) {
	s := state.NewEngineState()
	userID := types.UserID(1)
	symbol := "BTCUSDT"
	UpdatePosition(s, userID, symbol, types.Quantity(10), types.Price(50000), 0, 10)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		GetPosition(s, userID, symbol)
	}
}

func BenchmarkCheckLiquidation(b *testing.B) {
	pos := &types.Position{
		Size:              types.Quantity(10),
		Side:              0,
		EntryPrice:        types.Price(50000),
		Leverage:          10,
		InitialMargin:     50000,
		MaintenanceMargin: 5000,
		LiquidationPrice:  45000,
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		CheckLiquidation(pos, types.Price(49000))
	}
}
