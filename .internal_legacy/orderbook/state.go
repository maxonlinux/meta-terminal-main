package orderbook

import "sync"

type State struct {
	mu    sync.RWMutex
	books map[string]map[int8]*OrderBook
}

func NewState() *State {
	return &State{books: make(map[string]map[int8]*OrderBook)}
}

func (s *State) Get(symbol string, category int8) *OrderBook {
	s.mu.RLock()
	byCat := s.books[symbol]
	if byCat != nil {
		ob := byCat[category]
		s.mu.RUnlock()
		if ob != nil {
			return ob
		}
	} else {
		s.mu.RUnlock()
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.books[symbol] == nil {
		s.books[symbol] = make(map[int8]*OrderBook)
	}
	if s.books[symbol][category] == nil {
		s.books[symbol][category] = New()
	}
	return s.books[symbol][category]
}
