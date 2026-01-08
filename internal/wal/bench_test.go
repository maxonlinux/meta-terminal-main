package wal

import (
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkAppend(b *testing.B) {
	dir := b.TempDir()
	w, err := New(filepath.Join(dir, "wal"), 1024*1024, 64*1024)
	if err != nil {
		b.Fatalf("wal init failed: %v", err)
	}
	payload := []byte("test")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := w.Append(EventPriceTick, payload); err != nil {
			b.Fatalf("append failed: %v", err)
		}
	}
	_ = w.Close()
	_ = os.Remove(filepath.Join(dir, "wal"))
}
