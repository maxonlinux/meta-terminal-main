package portfolio

import "github.com/maxonlinux/meta-terminal-go/pkg/types"

// ExportBalances returns a copy of all balances.
func (s *Service) ExportBalances() []types.Balance {
	balances := make([]types.Balance, 0)
	for _, userBalances := range s.Balances {
		for _, balance := range userBalances {
			if balance == nil {
				continue
			}
			balances = append(balances, *balance)
		}
	}
	return balances
}

// ExportFundings returns a copy of all funding requests.
func (s *Service) ExportFundings() []types.FundingRequest {
	fundings := make([]types.FundingRequest, 0, len(s.Fundings))
	for _, funding := range s.Fundings {
		if funding == nil {
			continue
		}
		fundings = append(fundings, *funding)
	}
	return fundings
}

// ImportBalances replaces the balance state.
func (s *Service) ImportBalances(balances []types.Balance) {
	s.Balances = make(map[types.UserID]map[string]*types.Balance)
	for i := range balances {
		balance := balances[i]
		userBalances := s.Balances[balance.UserID]
		if userBalances == nil {
			userBalances = make(map[string]*types.Balance)
			s.Balances[balance.UserID] = userBalances
		}
		stored := balance
		userBalances[stored.Asset] = &stored
	}
}

// ImportFundings replaces the funding state.
func (s *Service) ImportFundings(fundings []types.FundingRequest) {
	s.Fundings = make(map[types.FundingID]*types.FundingRequest, len(fundings))
	for i := range fundings {
		funding := fundings[i]
		stored := funding
		s.Fundings[stored.ID] = &stored
	}
}

// ExportPositions returns a copy of all positions.
func (s *Service) ExportPositions() []types.Position {
	positions := make([]types.Position, 0)
	for _, userPositions := range s.Positions {
		for _, position := range userPositions {
			if position == nil {
				continue
			}
			positions = append(positions, *position)
		}
	}
	return positions
}

// ImportPositions replaces the position state.
func (s *Service) ImportPositions(positions []types.Position) {
	s.Positions = make(map[types.UserID]map[string]*types.Position)
	for i := range positions {
		position := positions[i]
		userPositions := s.Positions[position.UserID]
		if userPositions == nil {
			userPositions = make(map[string]*types.Position)
			s.Positions[position.UserID] = userPositions
		}
		stored := position
		userPositions[stored.Symbol] = &stored
	}
}
