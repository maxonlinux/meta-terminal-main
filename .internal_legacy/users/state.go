package users

import (
	"sync"

	"github.com/anomalyco/meta-terminal-go/internal/balance"
	"github.com/anomalyco/meta-terminal-go/internal/domain"
	"github.com/anomalyco/meta-terminal-go/internal/positions"
)

type UserState struct {
	Balances  map[string]*balance.Balance
	Positions map[string]*positions.Position
}

type State struct {
	mu    sync.RWMutex
	users map[domain.UserID]*UserState
}

func NewState() *State { return &State{users: make(map[domain.UserID]*UserState)} }

func (s *State) Get(userID domain.UserID) *UserState {
	s.mu.RLock()
	u := s.users[userID]
	s.mu.RUnlock()
	if u != nil {
		return u
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.users[userID] == nil {
		s.users[userID] = &UserState{
			Balances:  make(map[string]*balance.Balance),
			Positions: make(map[string]*positions.Position),
		}
	}
	return s.users[userID]
}

func (u *UserState) GetBalance(asset string) *balance.Balance {
	if u.Balances[asset] == nil {
		u.Balances[asset] = balance.New()
	}
	return u.Balances[asset]
}

func (u *UserState) GetPosition(symbol string) *positions.Position {
	if u.Positions[symbol] == nil {
		u.Positions[symbol] = positions.New(symbol)
	}
	return u.Positions[symbol]
}
