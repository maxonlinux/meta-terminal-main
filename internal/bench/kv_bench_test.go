package bench

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cockroachdb/pebble"
)

type kvBenchData struct {
	keys   []string
	values [][]byte
}

func newBenchData(count int, valueSize int) kvBenchData {
	keys := make([]string, count)
	values := make([][]byte, count)
	for i := 0; i < count; i++ {
		keys[i] = fmt.Sprintf("key-%d", i)
		payload := make([]byte, valueSize)
		_, _ = rand.Read(payload)
		values[i] = payload
	}
	return kvBenchData{keys: keys, values: values}
}

func benchPebbleWrite(b *testing.B, data kvBenchData, batchSize int) {
	path := filepath.Join(b.TempDir(), "pebble")
	db, err := pebble.Open(path, &pebble.Options{})
	if err != nil {
		b.Fatalf("open pebble: %v", err)
	}
	defer db.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		batch := db.NewBatch()
		for j := 0; j < batchSize; j++ {
			idx := (i*batchSize + j) % len(data.keys)
			if err := batch.Set([]byte(data.keys[idx]), data.values[idx], nil); err != nil {
				b.Fatalf("batch set: %v", err)
			}
		}
		if err := batch.Commit(pebble.Sync); err != nil {
			b.Fatalf("batch commit: %v", err)
		}
		batch.Close()
	}
}

func benchPebbleRead(b *testing.B, data kvBenchData) {
	path := filepath.Join(b.TempDir(), "pebble")
	db, err := pebble.Open(path, &pebble.Options{})
	if err != nil {
		b.Fatalf("open pebble: %v", err)
	}
	defer db.Close()

	batch := db.NewBatch()
	for i := range data.keys {
		if err := batch.Set([]byte(data.keys[i]), data.values[i], nil); err != nil {
			b.Fatalf("seed set: %v", err)
		}
	}
	if err := batch.Commit(pebble.Sync); err != nil {
		b.Fatalf("seed commit: %v", err)
	}
	batch.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		idx := i % len(data.keys)
		if _, closer, err := db.Get([]byte(data.keys[idx])); err == nil {
			_ = closer.Close()
		}
	}
}

func benchPebbleCheckpoint(b *testing.B, data kvBenchData) {
	path := filepath.Join(b.TempDir(), "pebble")
	db, err := pebble.Open(path, &pebble.Options{})
	if err != nil {
		b.Fatalf("open pebble: %v", err)
	}
	defer db.Close()

	batch := db.NewBatch()
	for i := range data.keys {
		if err := batch.Set([]byte(data.keys[i]), data.values[i], nil); err != nil {
			b.Fatalf("seed set: %v", err)
		}
	}
	if err := batch.Commit(pebble.Sync); err != nil {
		b.Fatalf("seed commit: %v", err)
	}
	batch.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		checkpointDir := filepath.Join(b.TempDir(), fmt.Sprintf("checkpoint-%d", i))
		if err := db.Checkpoint(checkpointDir); err != nil {
			b.Fatalf("checkpoint: %v", err)
		}
	}
}

func benchPebbleReplay(b *testing.B, data kvBenchData) {
	path := filepath.Join(b.TempDir(), "pebble")
	db, err := pebble.Open(path, &pebble.Options{})
	if err != nil {
		b.Fatalf("open pebble: %v", err)
	}
	defer db.Close()

	batch := db.NewBatch()
	for i := range data.keys {
		if err := batch.Set([]byte(data.keys[i]), data.values[i], nil); err != nil {
			b.Fatalf("seed set: %v", err)
		}
	}
	if err := batch.Commit(pebble.Sync); err != nil {
		b.Fatalf("seed commit: %v", err)
	}
	batch.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		iter, err := db.NewIter(nil)
		if err != nil {
			b.Fatalf("iter: %v", err)
		}
		for iter.First(); iter.Valid(); iter.Next() {
		}
		if err := iter.Close(); err != nil {
			b.Fatalf("iter close: %v", err)
		}
	}
}

func BenchmarkPebbleWrite1k(b *testing.B) {
	data := newBenchData(20000, 256)
	benchPebbleWrite(b, data, 1000)
}

func BenchmarkPebbleRead(b *testing.B) {
	data := newBenchData(20000, 256)
	benchPebbleRead(b, data)
}

func BenchmarkPebbleCheckpoint(b *testing.B) {
	data := newBenchData(20000, 256)
	benchPebbleCheckpoint(b, data)
}

func BenchmarkPebbleReplay(b *testing.B) {
	data := newBenchData(20000, 256)
	benchPebbleReplay(b, data)
}

func BenchmarkPebbleIterScan(b *testing.B) {
	data := newBenchData(20000, 256)
	path := filepath.Join(b.TempDir(), "pebble")
	db, err := pebble.Open(path, &pebble.Options{})
	if err != nil {
		b.Fatalf("open pebble: %v", err)
	}
	defer db.Close()

	batch := db.NewBatch()
	for i := range data.keys {
		if err := batch.Set([]byte(data.keys[i]), data.values[i], nil); err != nil {
			b.Fatalf("seed set: %v", err)
		}
	}
	if err := batch.Commit(pebble.Sync); err != nil {
		b.Fatalf("seed commit: %v", err)
	}
	batch.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		iter, err := db.NewIter(&pebble.IterOptions{})
		if err != nil {
			b.Fatalf("iter: %v", err)
		}
		for iter.First(); iter.Valid(); iter.Next() {
			_ = iter.Value()
		}
		if err := iter.Close(); err != nil {
			b.Fatalf("iter close: %v", err)
		}
	}
}

func BenchmarkPebbleBatch100(b *testing.B) {
	data := newBenchData(20000, 256)
	benchPebbleWrite(b, data, 100)
}

func BenchmarkPebbleBatch500(b *testing.B) {
	data := newBenchData(20000, 256)
	benchPebbleWrite(b, data, 500)
}

func BenchmarkPebbleBatch1000(b *testing.B) {
	data := newBenchData(20000, 256)
	benchPebbleWrite(b, data, 1000)
}

func BenchmarkPebbleBatch5000(b *testing.B) {
	data := newBenchData(20000, 256)
	benchPebbleWrite(b, data, 5000)
}

func TestMain(m *testing.M) {
	rand.Seed(time.Now().UnixNano())
	os.Exit(m.Run())
}
