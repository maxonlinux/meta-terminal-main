package outbox

import (
	"os"
	"testing"
)

func BenchmarkAppend(b *testing.B) {
	b.ReportAllocs()
	tmp, err := os.CreateTemp("", "outbox-bench-*.log")
	if err != nil {
		b.Fatalf("temp: %v", err)
	}
	path := tmp.Name()
	_ = tmp.Close()
	defer os.Remove(path)

	w, err := OpenWriter(path, 64*1024)
	if err != nil {
		b.Fatalf("open: %v", err)
	}
	defer w.Close()

	payload := make([]byte, 128)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := w.Append(1, payload); err != nil {
			b.Fatalf("append: %v", err)
		}
	}
}
