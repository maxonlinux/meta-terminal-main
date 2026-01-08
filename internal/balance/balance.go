package balance

import "sync"

type Balance struct {
	mu      sync.RWMutex
	buckets [3]int64
}

func New() *Balance { return &Balance{} }

func (b *Balance) Add(bucket int8, amount int64) {
	b.mu.Lock()
	b.buckets[bucket] += amount
	b.mu.Unlock()
}

func (b *Balance) Deduct(bucket int8, amount int64) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.buckets[bucket] < amount {
		return false
	}
	b.buckets[bucket] -= amount
	return true
}

func (b *Balance) Move(from, to int8, amount int64) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.buckets[from] < amount {
		return false
	}
	b.buckets[from] -= amount
	b.buckets[to] += amount
	return true
}

func (b *Balance) Get(bucket int8) int64 {
	b.mu.RLock()
	v := b.buckets[bucket]
	b.mu.RUnlock()
	return v
}

func (b *Balance) Snapshot() [3]int64 {
	b.mu.RLock()
	v := b.buckets
	b.mu.RUnlock()
	return v
}

func (b *Balance) Restore(v [3]int64) {
	b.mu.Lock()
	b.buckets = v
	b.mu.Unlock()
}
