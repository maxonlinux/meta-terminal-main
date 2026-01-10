package history

import (
	"context"

	"github.com/anomalyco/meta-terminal-go/internal/persistence/duckdb"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

type Config struct {
	DBPath string
}

type Service struct {
	repo *duckdb.Repository
}

func New(cfg Config) (*Service, error) {
	db, err := duckdb.New(duckdb.Config{Path: cfg.DBPath})
	if err != nil {
		return nil, err
	}

	return &Service{
		repo: db,
	}, nil
}

func (s *Service) Close() error {
	return s.repo.Close()
}

func (s *Service) GetOrderHistory(ctx context.Context, userID types.UserID, symbol string, limit int) ([]*types.Order, error) {
	if limit <= 0 {
		limit = 100
	}
	return s.repo.GetOrders(userID, symbol, limit)
}

func (s *Service) GetTradeHistory(ctx context.Context, userID types.UserID, symbol string, limit int) ([]*types.Trade, error) {
	if limit <= 0 {
		limit = 100
	}
	return s.repo.GetTrades(userID, symbol, limit)
}

func (s *Service) GetRPNLHistory(ctx context.Context, userID types.UserID, symbol string, limit int) ([]*types.RPNLEvent, error) {
	if limit <= 0 {
		limit = 100
	}
	return nil, nil
}

func (s *Service) GetPositionsFromDB(ctx context.Context, userID types.UserID, symbol string) ([]types.Position, error) {
	return s.repo.GetPositions(userID, symbol)
}
