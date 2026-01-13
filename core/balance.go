package core

// BalanceService manages user balances
// THIS IS THE SOURCE OF TRUTH for all user balances
// WHY: Centralized balance management ensures consistency across the system
// WHY: Event-driven design maintains zero-lock architecture
type BalanceService struct {
	// THIS IS THE SOURCE OF TRUTH for balances
	// balances[userID][symbol] -> Balance
	balances map[UserID]map[string]*Balance
}

// NewBalanceService creates a new balance service
func NewBalanceService() *BalanceService {
	return &BalanceService{
		balances: make(map[UserID]map[string]*Balance),
	}
}

// Get returns user's balance for a symbol
// Returns nil if not found (caller should handle)
func (s *BalanceService) Get(userID UserID, symbol string) *Balance {
	if userBalances := s.balances[userID]; userBalances != nil {
		return userBalances[symbol]
	}
	return nil
}

// EnsureBalance creates balance entry if it doesn't exist
// WHY: Called before any balance operation to ensure balance exists
func (s *BalanceService) EnsureBalance(userID UserID, symbol string) *Balance {
	if s.balances[userID] == nil {
		s.balances[userID] = make(map[string]*Balance)
	}
	if s.balances[userID][symbol] == nil {
		s.balances[userID][symbol] = &Balance{
			UserID:    userID,
			Symbol:    symbol,
			Available: 0,
			Locked:    0,
			Margin:    0,
		}
	}
	return s.balances[userID][symbol]
}

// Reserve moves amount from Available to Locked
// Called before placing an order
// WHY: Reserves funds for order execution, prevents double-spending
// ERROR: Returns ErrInsufficientBalance if not enough available
func (s *BalanceService) Reserve(userID UserID, symbol string, amount Quantity) error {
	balance := s.EnsureBalance(userID, symbol)
	if balance.Available < amount {
		return ErrInsufficientBalance
	}
	balance.Available -= amount
	balance.Locked += amount
	return nil
}

// Release moves amount from Locked back to Available
// Called when order is cancelled or reduced
// WHY: Returns reserved funds when order no longer needs them
func (s *BalanceService) Release(userID UserID, symbol string, amount Quantity) {
	balance := s.Get(userID, symbol)
	if balance == nil {
		return
	}
	released := min(amount, balance.Locked)
	balance.Locked -= released
	balance.Available += released
}

// Deduct removes amount from Locked (order was filled)
// Called when trade executes
// WHY: Finalizes balance change after order fill
func (s *BalanceService) Deduct(userID UserID, symbol string, amount Quantity) {
	balance := s.Get(userID, symbol)
	if balance == nil {
		return
	}
	deducted := min(amount, balance.Locked)
	balance.Locked -= deducted
}

// AddCredit adds amount to Available (trade profit, deposit)
// Called on trade settlement or deposit
func (s *BalanceService) AddCredit(userID UserID, symbol string, amount Quantity) {
	balance := s.EnsureBalance(userID, symbol)
	balance.Available += amount
}

// AddMargin adds amount to Margin (used for position maintenance)
func (s *BalanceService) AddMargin(userID UserID, symbol string, amount Quantity) {
	balance := s.EnsureBalance(userID, symbol)
	balance.Margin += amount
}

// UseMargin moves amount from Margin to Locked
func (s *BalanceService) UseMargin(userID UserID, symbol string, amount Quantity) error {
	balance := s.EnsureBalance(userID, symbol)
	if balance.Margin < amount {
		return ErrInsufficientBalance
	}
	balance.Margin -= amount
	balance.Locked += amount
	return nil
}

// ReleaseMargin moves amount from Locked back to Margin
func (s *BalanceService) ReleaseMargin(userID UserID, symbol string, amount Quantity) {
	balance := s.Get(userID, symbol)
	if balance == nil {
		return
	}
	released := min(amount, balance.Locked)
	balance.Locked -= released
	balance.Margin += released
}

// TransferMargin moves margin between symbols for same user
func (s *BalanceService) TransferMargin(userID UserID, fromSymbol, toSymbol string, amount Quantity) error {
	fromBalance := s.Get(userID, fromSymbol)
	if fromBalance == nil || fromBalance.Margin < amount {
		return ErrInsufficientBalance
	}
	fromBalance.Margin -= amount

	toBalance := s.EnsureBalance(userID, toSymbol)
	toBalance.Margin += amount
	return nil
}

// Deposit adds funds to user's balance (external deposits)
func (s *BalanceService) Deposit(userID UserID, symbol string, amount Quantity) {
	balance := s.EnsureBalance(userID, symbol)
	balance.Available += amount
}

// Withdraw removes funds from user's balance (external withdrawal)
func (s *BalanceService) Withdraw(userID UserID, symbol string, amount Quantity) error {
	balance := s.EnsureBalance(userID, symbol)
	if balance.Available < amount {
		return ErrInsufficientBalance
	}
	balance.Available -= amount
	return nil
}

// GetAvailable returns total available for trading (Available + Margin)
func (s *BalanceService) GetAvailable(userID UserID, symbol string) Quantity {
	balance := s.Get(userID, symbol)
	if balance == nil {
		return 0
	}
	return balance.Available + balance.Margin
}

// GetTotal returns total balance (Available + Locked + Margin)
func (s *BalanceService) GetTotal(userID UserID, symbol string) Quantity {
	balance := s.Get(userID, symbol)
	if balance == nil {
		return 0
	}
	return balance.Available + balance.Locked + balance.Margin
}

// GetLocked returns locked amount
func (s *BalanceService) GetLocked(userID UserID, symbol string) Quantity {
	balance := s.Get(userID, symbol)
	if balance == nil {
		return 0
	}
	return balance.Locked
}

// GetMargin returns margin amount
func (s *BalanceService) GetMargin(userID UserID, symbol string) Quantity {
	balance := s.Get(userID, symbol)
	if balance == nil {
		return 0
	}
	return balance.Margin
}

// Count returns total number of balances
func (s *BalanceService) Count() int {
	total := 0
	for _, balances := range s.balances {
		total += len(balances)
	}
	return total
}
