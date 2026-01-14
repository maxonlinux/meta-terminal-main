package types

// UserBalance represents user's total balance for an asset
type Balance struct {
	UserID    UserID
	Asset     string
	Available Quantity
	Locked    Quantity
	Margin    Quantity
}
