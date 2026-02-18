package types

// FundingType defines funding request direction.
type FundingType string

const (
	FundingTypeDeposit    FundingType = "DEPOSIT"
	FundingTypeWithdrawal FundingType = "WITHDRAWAL"
)

// FundingStatus describes funding request lifecycle.
type FundingStatus string

const (
	FundingStatusPending   FundingStatus = "PENDING"
	FundingStatusCompleted FundingStatus = "COMPLETED"
	FundingStatusCanceled  FundingStatus = "CANCELED"
)

// FundingCreatedBy describes who initiated the funding request.
type FundingCreatedBy string

const (
	FundingCreatedByUser     FundingCreatedBy = "USER"
	FundingCreatedByAdmin    FundingCreatedBy = "ADMIN"
	FundingCreatedByPlatform FundingCreatedBy = "PLATFORM"
)

// FundingRequest tracks deposit/withdraw flows.
type FundingRequest struct {
	ID          FundingID
	UserID      UserID
	Type        FundingType
	Status      FundingStatus
	Asset       string
	Amount      Quantity
	Destination string
	CreatedBy   FundingCreatedBy
	Message     string
	CreatedAt   uint64
	UpdatedAt   uint64
}
