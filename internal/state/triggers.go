package state

import (
	"sync"

	"github.com/anomalyco/meta-terminal-go/internal/trigger"
)

type Triggers struct {
	mu    sync.RWMutex
	bySym map[string]*trigger.Monitor
}

func NewTriggers() *Triggers {
	return &Triggers{bySym: make(map[string]*trigger.Monitor)}
}

func (s *Triggers) Get(symbol string) *trigger.Monitor {
	s.mu.RLock()
	mon := s.bySym[symbol]
	s.mu.RUnlock()
	if mon != nil {
		return mon
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.bySym[symbol] == nil {
		s.bySym[symbol] = trigger.NewMonitor()
	}
	return s.bySym[symbol]
}

func (s *Triggers) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bySym = make(map[string]*trigger.Monitor)
}
