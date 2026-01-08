package balance

import (
	"sync"

	"github.com/anomalyco/meta-terminal-go/internal/domain"
)

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

type State struct {
	mu       sync.RWMutex
	balances map[domain.UserID]map[string]*Balance
}

func NewState() *State { return &State{balances: make(map[domain.UserID]map[string]*Balance)} }

func (s *State) Get(userID domain.UserID, asset string) *Balance {
	s.mu.RLock()
	byAsset := s.balances[userID]
	if byAsset != nil {
		b := byAsset[asset]
		s.mu.RUnlock()
		if b != nil {
			return b
		}
	} else {
		s.mu.RUnlock()
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.balances[userID] == nil {
		s.balances[userID] = make(map[string]*Balance)
	}
	if s.balances[userID][asset] == nil {
		s.balances[userID][asset] = New()
	}
	return s.balances[userID][asset]
}
