package portfolio

import (
	"errors"

	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/snowflake"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/maxonlinux/meta-terminal-go/pkg/utils"
)

var (
	ErrFundingNotFound    = errors.New("funding request not found")
	ErrFundingNotPending  = errors.New("funding request is not pending")
	ErrFundingAmountZero  = errors.New("funding amount must be positive")
	ErrFundingDestination = errors.New("funding destination required")
	ErrFundingInvalidType = errors.New("invalid funding type")
)

// CreateWithdrawal locks available funds and stores a pending request.
func (s *Service) CreateWithdrawal(userID types.UserID, asset string, amount types.Quantity, destination string, createdBy types.FundingCreatedBy, message string) (*types.FundingRequest, error) {
	if amount.Sign() <= 0 {
		return nil, ErrFundingAmountZero
	}
	if destination == "" {
		return nil, ErrFundingDestination
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.reserveLocked(userID, asset, amount); err != nil {
		return nil, err
	}

	request := s.newFundingRequest(userID, asset, amount, destination, createdBy, message, types.FundingTypeWithdrawal)
	s.addFunding(request)
	return request, nil
}

// CreateDeposit stores a pending deposit request.
func (s *Service) CreateDeposit(userID types.UserID, asset string, amount types.Quantity, destination string, createdBy types.FundingCreatedBy, message string) (*types.FundingRequest, error) {
	if amount.Sign() <= 0 {
		return nil, ErrFundingAmountZero
	}
	if destination == "" {
		return nil, ErrFundingDestination
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	request := s.newFundingRequest(userID, asset, amount, destination, createdBy, message, types.FundingTypeDeposit)
	s.addFunding(request)
	return request, nil
}

// ApproveFunding completes a pending funding request.
func (s *Service) ApproveFunding(id types.FundingID) (*types.FundingRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	request, err := s.pendingFundingLocked(id)
	if err != nil {
		return nil, err
	}

	if err := s.applyFundingApproval(request); err != nil {
		return nil, err
	}

	s.transitionFunding(request, types.FundingStatusCompleted)
	return request, nil
}

// RejectFunding cancels a pending funding request.
func (s *Service) RejectFunding(id types.FundingID) (*types.FundingRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	request, err := s.pendingFundingLocked(id)
	if err != nil {
		return nil, err
	}

	if request.Type == types.FundingTypeWithdrawal {
		s.releaseLocked(request.UserID, request.Asset, request.Amount)
	}

	s.transitionFunding(request, types.FundingStatusCanceled)
	return request, nil
}

// pendingFundingLocked expects the portfolio mutex to be held.
func (s *Service) pendingFundingLocked(id types.FundingID) (*types.FundingRequest, error) {
	request, ok := s.Fundings[id]
	if !ok {
		return nil, ErrFundingNotFound
	}
	if request.Status != types.FundingStatusPending {
		return nil, ErrFundingNotPending
	}
	return request, nil
}

func (s *Service) applyFundingApproval(request *types.FundingRequest) error {
	switch request.Type {
	case types.FundingTypeDeposit:
		s.adjustAvailable(request.UserID, request.Asset, request.Amount)
		return nil
	case types.FundingTypeWithdrawal:
		s.adjustLocked(request.UserID, request.Asset, math.Neg(request.Amount))
		return nil
	default:
		return ErrFundingInvalidType
	}
}

func (s *Service) newFundingRequest(userID types.UserID, asset string, amount types.Quantity, destination string, createdBy types.FundingCreatedBy, message string, kind types.FundingType) *types.FundingRequest {
	now := utils.NowNano()
	return &types.FundingRequest{
		ID:          types.FundingID(snowflake.Next()),
		UserID:      userID,
		Type:        kind,
		Status:      types.FundingStatusPending,
		Asset:       asset,
		Amount:      amount,
		Destination: destination,
		CreatedBy:   createdBy,
		Message:     message,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func (s *Service) transitionFunding(request *types.FundingRequest, status types.FundingStatus) {
	request.Status = status
	request.UpdatedAt = utils.NowNano()
}

func (s *Service) addFunding(request *types.FundingRequest) {
	if s.Fundings == nil {
		s.Fundings = make(map[types.FundingID]*types.FundingRequest)
	}
	s.Fundings[request.ID] = request
}
