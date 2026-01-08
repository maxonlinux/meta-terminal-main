package balance

import (
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/domain"
)

func BenchmarkBalanceAdd(b *testing.B) {
	x := New()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		x.Add(constants.BUCKET_AVAILABLE, 1)
	}
}

func BenchmarkBalanceDeduct(b *testing.B) {
	x := New()
	x.Add(constants.BUCKET_AVAILABLE, int64(b.N+1))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = x.Deduct(constants.BUCKET_AVAILABLE, 1)
	}
}

func BenchmarkBalanceMove(b *testing.B) {
	x := New()
	x.Add(constants.BUCKET_AVAILABLE, int64(b.N+1))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = x.Move(constants.BUCKET_AVAILABLE, constants.BUCKET_LOCKED, 1)
	}
}

func BenchmarkStateGet(b *testing.B) {
	s := NewState()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = s.Get(domain.UserID(i%1024), "USDT")
	}
}
