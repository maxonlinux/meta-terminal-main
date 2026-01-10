package snowflake

import (
	"sync/atomic"
	"testing"
)

func BenchmarkNext(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Next()
	}
}

func TestUniqueIDs(t *testing.T) {
	ids := make(map[int64]bool)

	for i := 0; i < 100000; i++ {
		id := Next()
		if ids[id] {
			t.Errorf("Duplicate ID generated: %d", id)
		}
		ids[id] = true
	}

	if len(ids) != 100000 {
		t.Errorf("Expected 100000 unique IDs, got %d", len(ids))
	}
}

func TestOrder(t *testing.T) {
	var lastID int64

	for i := 0; i < 1000; i++ {
		id := Next()
		if id <= lastID {
			t.Errorf("IDs not in increasing order: last=%d, current=%d", lastID, id)
		}
		lastID = id
	}
}

func TestParallel(t *testing.T) {
	var n int64 = 0
	var sum int64 = 0

	done := make(chan bool)
	for i := 0; i < 100; i++ {
		go func() {
			for j := 0; j < 1000; j++ {
				id := Next()
				atomic.AddInt64(&n, 1)
				atomic.AddInt64(&sum, id)
			}
			done <- true
		}()
	}

	for i := 0; i < 100; i++ {
		<-done
	}

	if n != 100000 {
		t.Errorf("Expected 100000 IDs, got %d", n)
	}
}
