package balance

import "testing"

func BenchmarkMove(b *testing.B) {
	bal := New()
	bal.Add(0, 1_000_000_000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bal.Move(0, 1, 1)
		bal.Move(1, 0, 1)
	}
}
