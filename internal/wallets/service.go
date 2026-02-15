package wallets

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

type Service struct {
	repo *Repository
	rng  *rand.Rand
}

func NewService(repo *Repository) *Service {
	return &Service{
		repo: repo,
		rng:  rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (s *Service) ListWallets() ([]Wallet, error) {
	return s.repo.ListWallets()
}

func (s *Service) CreateWallet(wallet Wallet) (int64, error) {
	if !wallet.IsActive {
		wallet.IsActive = true
	}
	return s.repo.CreateWallet(wallet)
}

func (s *Service) UpdateWallet(id int64, wallet Wallet) error {
	return s.repo.UpdateWallet(id, wallet)
}

func (s *Service) ListUserWallets(userID types.UserID, activeOnly bool) ([]UserWallet, error) {
	return s.repo.ListUserWallets(userID, activeOnly)
}

func (s *Service) AssignWallet(userID types.UserID, walletID int64, assignedBy string) error {
	wallet, err := s.repo.GetWallet(walletID)
	if err != nil {
		return err
	}
	if wallet == nil {
		return fmt.Errorf("wallet not found")
	}
	return s.repo.AssignWallet(userID, *wallet, assignedBy)
}

func (s *Service) GetUserWallet(userID types.UserID, walletID int64) (*UserWallet, error) {
	return s.repo.GetUserWallet(userID, walletID)
}

// AssignDefaultWallets assigns one random standard wallet per network.
func (s *Service) AssignDefaultWallets(userID types.UserID) error {
	wallets, err := s.repo.ListStandardWallets()
	if err != nil {
		return err
	}
	if len(wallets) == 0 {
		return nil
	}

	byNetwork := make(map[string][]Wallet)
	for _, wallet := range wallets {
		byNetwork[wallet.Network] = append(byNetwork[wallet.Network], wallet)
	}

	for network, pool := range byNetwork {
		if len(pool) == 0 {
			continue
		}
		selected := pool[s.rng.Intn(len(pool))]
		if err := s.repo.AssignWallet(userID, selected, "system"); err != nil {
			return fmt.Errorf("assign wallet network=%s: %w", network, err)
		}
	}

	return nil
}

func (s *Service) CountWallets() (int, error) {
	return s.repo.CountWallets()
}
