package balance

import (
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/types"
)

func BenchmarkAdd(b *testing.B) {
	s := state.NewEngineState()
	userID := types.UserID(1)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		Add(s, userID, "USDT", types.BUCKET_AVAILABLE, 1000)
	}
}

func BenchmarkDeduct(b *testing.B) {
	s := state.NewEngineState()
	userID := types.UserID(1)
	Add(s, userID, "USDT", types.BUCKET_AVAILABLE, 1000000)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		Deduct(s, userID, "USDT", types.BUCKET_AVAILABLE, 1000)
	}
}

func BenchmarkMove(b *testing.B) {
	s := state.NewEngineState()
	userID := types.UserID(1)
	Add(s, userID, "USDT", types.BUCKET_AVAILABLE, 1000000)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		Move(s, userID, "USDT", types.BUCKET_AVAILABLE, types.BUCKET_LOCKED, 1000)
	}
}

func BenchmarkGet(b *testing.B) {
	s := state.NewEngineState()
	userID := types.UserID(1)
	Add(s, userID, "USDT", types.BUCKET_AVAILABLE, 1000000)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		Get(s, userID, "USDT", types.BUCKET_AVAILABLE)
	}
}
