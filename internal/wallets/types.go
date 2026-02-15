package wallets

import "github.com/maxonlinux/meta-terminal-go/pkg/types"

type Wallet struct {
	ID        int64
	Name      string
	Address   string
	Network   string
	Currency  string
	IsCustom  bool
	IsActive  bool
	CreatedAt uint64
	UpdatedAt uint64
}

type UserWallet struct {
	UserID     types.UserID
	WalletID   int64
	Name       string
	Address    string
	Network    string
	Currency   string
	IsCustom   bool
	IsActive   bool
	AssignedAt uint64
	AssignedBy string
}
