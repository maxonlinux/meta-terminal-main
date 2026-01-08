package users

import (
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/domain"
)

func BenchmarkUsersGet(b *testing.B) {
	s := NewState()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = s.Get(domain.UserID(i % 4096))
	}
}
