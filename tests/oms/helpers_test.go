package oms_test

import (
	"github.com/anomalyco/meta-terminal-go/internal/clearing"
	"github.com/anomalyco/meta-terminal-go/internal/oms"
	"github.com/anomalyco/meta-terminal-go/internal/portfolio"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func newService() (*oms.Service, *portfolio.Service) {
	port := portfolio.New(portfolio.Config{})
	clear := clearing.New(port)
	svc, _ := oms.New(oms.Config{}, port, clear)
	return svc, port
}

func newServiceWithClearing(clearing oms.Clearing) (*oms.Service, *portfolio.Service) {
	port := portfolio.New(portfolio.Config{})
	svc, _ := oms.New(oms.Config{}, port, clearing)
	return svc, port
}

func setBalance(port *portfolio.Service, userID types.UserID, asset string, available, locked, margin int64) {
	if port.Balances[userID] == nil {
		port.Balances[userID] = make(map[string]*types.UserBalance)
	}
	port.Balances[userID][asset] = &types.UserBalance{
		Asset:     asset,
		Available: available,
		Locked:    locked,
		Margin:    margin,
	}
}

func setPosition(port *portfolio.Service, userID types.UserID, symbol string, size int64, side int8, entryPrice int64, leverage int8) {
	if port.Positions[userID] == nil {
		port.Positions[userID] = make(map[string]*types.Position)
	}
	port.Positions[userID][symbol] = &types.Position{
		Symbol:     symbol,
		Size:       size,
		Side:       side,
		EntryPrice: entryPrice,
		Leverage:   leverage,
	}
}
