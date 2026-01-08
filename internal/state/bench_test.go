package state

import (
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func BenchmarkUsersGet(b *testing.B) {
	state := NewUsers()
	b.ReportAllocs()

	for i := 0; b.Loop(); i++ {
		_ = state.Get(types.UserID(i % 4096))
	}
}

func BenchmarkUsersGetBalance(b *testing.B) {
	state := NewUsers()
	b.ReportAllocs()

	for i := 0; b.Loop(); i++ {
		_ = state.GetBalance(types.UserID(i%1024), "USDT")
	}
}
