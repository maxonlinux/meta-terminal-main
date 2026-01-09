package persistence

import (
	"encoding/binary"
	"os"
	"testing"
)

func benchmarkWALSave(b *testing.B, async bool) {
	tmpDir, err := os.MkdirTemp("", "wal-bench")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := New(tmpDir)
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()

	data := make([]byte, 256)
	binary.BigEndian.PutUint32(data[:4], 12345)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		txId := uint64(1)
		for pb.Next() {
			store.Save("oms", "BTCUSDT", data, txId)
			txId++
		}
	})
}

func BenchmarkWALSave(b *testing.B) {
	benchmarkWALSave(b, false)
}

func BenchmarkWALSaveAsync(b *testing.B) {
	benchmarkWALSave(b, true)
}

func BenchmarkWALLoad(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "wal-bench")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := New(tmpDir)
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()

	data := make([]byte, 256)
	binary.BigEndian.PutUint32(data[:4], 12345)

	for i := uint64(1); i <= 1000; i++ {
		store.Save("oms", "BTCUSDT", data, i)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			store.Load("oms", "BTCUSDT")
		}
	})
}

func BenchmarkWALSaveTx(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "wal-bench")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := New(tmpDir)
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		txId := uint64(1)
		for pb.Next() {
			store.SaveTx(txId, "oms", "PLACED")
			txId++
		}
	})
}

func BenchmarkWALConcurrentSave(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "wal-bench")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := New(tmpDir)
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()

	data := make([]byte, 256)
	binary.BigEndian.PutUint32(data[:4], 12345)

	b.ResetTimer()
	b.SetParallelism(10)
	b.RunParallel(func(pb *testing.PB) {
		txId := uint64(1)
		for pb.Next() {
			store.Save("oms", "BTCUSDT", data, txId)
			txId++
		}
	})
}
