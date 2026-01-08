package orderbook

import "sync"

type State struct {
	mu    sync.RWMutex
	books map[string]map[int8]*OrderBook
	idGen IDGenerator
}

func NewState() *State {
	return NewStateWithIDGenerator(nil)
}

func NewStateWithIDGenerator(idGen IDGenerator) *State {
	return &State{books: make(map[string]map[int8]*OrderBook), idGen: idGen}
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
		s.books[symbol][category] = NewWithIDGenerator(s.idGen)
	}
	return s.books[symbol][category]
}

func (s *State) SetIDGenerator(idGen IDGenerator) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.idGen = idGen
	for _, byCat := range s.books {
		for _, ob := range byCat {
			if ob != nil {
				ob.SetIDGenerator(idGen)
			}
		}
	}
}

func (s *State) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.books = make(map[string]map[int8]*OrderBook)
}
