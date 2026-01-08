package state

import (
	"maps"
	"sync"

	"github.com/anomalyco/meta-terminal-go/internal/balance"
	"github.com/anomalyco/meta-terminal-go/internal/positions"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

type UserState struct {
	Balances  map[string]*balance.Balance
	Positions map[string]*positions.Position
}

type Users struct {
	mu    sync.RWMutex
	users map[types.UserID]*UserState
}

func NewUsers() *Users { return &Users{users: make(map[types.UserID]*UserState)} }

func (s *Users) Get(userID types.UserID) *UserState {
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

func (s *Users) GetBalance(userID types.UserID, asset string) *balance.Balance {
	user := s.Get(userID)
	if user.Balances[asset] == nil {
		user.Balances[asset] = balance.New()
	}
	return user.Balances[asset]
}

func (s *Users) GetPosition(userID types.UserID, symbol string) *positions.Position {
	user := s.Get(userID)
	if user.Positions[symbol] == nil {
		user.Positions[symbol] = positions.New(symbol)
	}
	return user.Positions[symbol]
}

func (s *Users) AllUsers() map[types.UserID]*UserState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[types.UserID]*UserState, len(s.users))
	maps.Copy(out, s.users)
	return out
}

func (s *Users) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.users = make(map[types.UserID]*UserState)
}
