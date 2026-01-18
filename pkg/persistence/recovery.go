package persistence

import (
	"errors"

	"github.com/maxonlinux/meta-terminal-go/internal/clearing"
	"github.com/maxonlinux/meta-terminal-go/internal/portfolio"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

// StateProvider exposes engine state for snapshotting.
type StateProvider interface {
	ExportOrders() []types.Order
	ExportBalances() []types.Balance
	ExportPositions() []types.Position
	ExportFundings() []types.FundingRequest
}

// StateRestorer applies restored engine state.
type StateRestorer interface {
	ImportOrders(orders []types.Order)
	ImportBalances(balances []types.Balance)
	ImportPositions(positions []types.Position)
	ImportFundings(fundings []types.FundingRequest)
}

// SaveSnapshot writes a persistence snapshot from the provided state.
func SaveSnapshot(path string, lastSeq uint64, provider StateProvider) error {
	if provider == nil {
		return errors.New("snapshot provider is required")
	}
	if path == "" {
		return errors.New("snapshot path is required")
	}
	snapshot := Snapshot{
		LastSeq:   lastSeq,
		Orders:    provider.ExportOrders(),
		Balances:  provider.ExportBalances(),
		Positions: provider.ExportPositions(),
		Fundings:  provider.ExportFundings(),
	}
	return WriteSnapshot(path, snapshot)
}

// LoadSnapshot loads a snapshot and applies it to the restorer.
func LoadSnapshot(path string, restorer StateRestorer) (Snapshot, error) {
	if restorer == nil {
		return Snapshot{}, errors.New("snapshot restorer is required")
	}
	if path == "" {
		return Snapshot{}, errors.New("snapshot path is required")
	}
	snapshot, err := LoadSnapshotFile(path)
	if err != nil {
		return Snapshot{}, err
	}
	restorer.ImportOrders(snapshot.Orders)
	restorer.ImportBalances(snapshot.Balances)
	restorer.ImportPositions(snapshot.Positions)
	restorer.ImportFundings(snapshot.Fundings)
	return snapshot, nil
}

// ReplayFromSnapshot replays events after the snapshot sequence.
func ReplayFromSnapshot(path string, source ReplaySource, restorer StateRestorer) error {
	if source == nil {
		return errors.New("replay source is required")
	}
	if restorer == nil {
		return errors.New("snapshot restorer is required")
	}
	// Snapshot path must be provided for recovery.
	if path == "" {
		return errors.New("snapshot path is required")
	}

	snapshot, err := LoadSnapshotFile(path)
	if err != nil {
		return err
	}

	orders, fundings, err := replayState(snapshot, source)
	if err != nil {
		return err
	}

	restorer.ImportOrders(orders)
	restorer.ImportBalances(snapshot.Balances)
	restorer.ImportPositions(snapshot.Positions)
	restorer.ImportFundings(fundings)
	return nil
}

// ReplayToSnapshot replays events and writes a merged snapshot.
func ReplayToSnapshot(path string, outputPath string, source ReplaySource, lastSeq uint64) error {
	if source == nil {
		return errors.New("replay source is required")
	}
	if path == "" {
		return errors.New("snapshot path is required")
	}
	if outputPath == "" {
		return errors.New("output path is required")
	}

	snapshot, err := LoadSnapshotFile(path)
	if err != nil {
		return err
	}

	orders, fundings, err := replayState(snapshot, source)
	if err != nil {
		return err
	}

	merged := Snapshot{
		LastSeq:   pickReplaySeq(lastSeq, snapshot.LastSeq),
		Orders:    orders,
		Balances:  snapshot.Balances,
		Positions: snapshot.Positions,
		Fundings:  fundings,
		Metadata:  snapshot.Metadata,
	}
	return WriteSnapshot(outputPath, merged)
}

// ReplayWALToSnapshot replays WAL events into a merged snapshot.
func ReplayWALToSnapshot(snapshotPath string, walPath string, outputPath string) error {
	if snapshotPath == "" {
		return errors.New("snapshot path is required")
	}
	if walPath == "" {
		return errors.New("wal path is required")
	}
	if outputPath == "" {
		return errors.New("output path is required")
	}
	// Snapshot replay relies on balances/positions to apply trades.

	snapshot, err := LoadSnapshotFile(snapshotPath)
	if err != nil {
		return err
	}

	portfolioService := portfolio.New(nil)
	portfolioService.ImportBalances(snapshot.Balances)
	portfolioService.ImportPositions(snapshot.Positions)
	// Clearing service is required to apply trade effects during replay.
	clearingSvc := clearing.New(portfolioService)

	orders := make(map[types.OrderID]*types.Order, len(snapshot.Orders))
	for _, order := range snapshot.Orders {
		stored := order
		orders[stored.ID] = &stored
	}

	matches := make(map[types.TradeID]*types.Match)
	lastSeq, err := ReplayWAL(walPath, snapshot.LastSeq+1, func(record Record) error {
		if record.Order != nil {
			order := record.Order.Order
			stored := order
			orders[stored.ID] = &stored
			return nil
		}
		if record.Trade == nil {
			return nil
		}

		trade := record.Trade.Trade
		match := matches[trade.MatchID]
		if match == nil {
			match = &types.Match{
				ID:        trade.MatchID,
				Symbol:    trade.Symbol,
				Category:  trade.Category,
				Price:     trade.Price,
				Quantity:  trade.Quantity,
				Timestamp: trade.Timestamp,
			}
			matches[trade.MatchID] = match
		}

		order := orders[trade.OrderID]
		if order == nil {
			order = &types.Order{
				ID:       trade.OrderID,
				UserID:   trade.UserID,
				Symbol:   trade.Symbol,
				Category: trade.Category,
				Side:     trade.Side,
			}
			orders[trade.OrderID] = order
		}

		if trade.IsMaker {
			match.MakerOrder = order
		} else {
			match.TakerOrder = order
		}

		if match.MakerOrder != nil && match.TakerOrder != nil {
			clearingSvc.ExecuteTrade(match)
			delete(matches, trade.MatchID)
		}
		return nil
	})
	if err != nil {
		return err
	}

	mergedOrders := make([]types.Order, 0, len(orders))
	for _, order := range orders {
		mergedOrders = append(mergedOrders, *order)
	}

	merged := Snapshot{
		LastSeq:   pickReplaySeq(lastSeq, snapshot.LastSeq),
		Orders:    mergedOrders,
		Balances:  portfolioService.ExportBalances(),
		Positions: portfolioService.ExportPositions(),
		Fundings:  snapshot.Fundings,
		Metadata:  snapshot.Metadata,
	}
	return WriteSnapshot(outputPath, merged)
}

// ReplayToSnapshotFromSeq replays events into a fresh snapshot.
func ReplayToSnapshotFromSeq(outputPath string, fromSeq uint64, source ReplaySource, lastSeq uint64) error {
	if source == nil {
		return errors.New("replay source is required")
	}
	if outputPath == "" {
		return errors.New("output path is required")
	}
	// fromSeq must be explicit to avoid replaying from zero.
	if fromSeq == 0 {
		return errors.New("from seq is required")
	}

	orders, fundings, err := replayStateFromSeq(fromSeq, source)
	if err != nil {
		return err
	}

	snapshot := Snapshot{
		LastSeq:   pickReplaySeq(lastSeq, fromSeq-1),
		Orders:    orders,
		Balances:  nil,
		Positions: nil,
		Fundings:  fundings,
	}
	return WriteSnapshot(outputPath, snapshot)
}

func replayState(snapshot Snapshot, source ReplaySource) ([]types.Order, []types.FundingRequest, error) {
	orders := make(map[types.OrderID]types.Order, len(snapshot.Orders))
	for _, order := range snapshot.Orders {
		orders[order.ID] = order
	}
	fundings := make(map[types.FundingID]types.FundingRequest, len(snapshot.Fundings))
	for _, funding := range snapshot.Fundings {
		fundings[funding.ID] = funding
	}
	return replayStateFromSeqWithMap(snapshot.LastSeq+1, source, orders, fundings)
}

func replayStateFromSeq(fromSeq uint64, source ReplaySource) ([]types.Order, []types.FundingRequest, error) {
	return replayStateFromSeqWithMap(fromSeq, source, make(map[types.OrderID]types.Order), make(map[types.FundingID]types.FundingRequest))
}

func pickReplaySeq(proposed uint64, fallback uint64) uint64 {
	if proposed == 0 {
		return fallback
	}
	return proposed
}

func replayStateFromSeqWithMap(fromSeq uint64, source ReplaySource, orders map[types.OrderID]types.Order, fundings map[types.FundingID]types.FundingRequest) ([]types.Order, []types.FundingRequest, error) {
	err := Replay(source, fromSeq, func(record Record) error {
		if record.Order != nil {
			orders[record.Order.Order.ID] = record.Order.Order
			return nil
		}
		if record.Funding != nil {
			fundings[record.Funding.Funding.ID] = record.Funding.Funding
			return nil
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	mergedOrders := make([]types.Order, 0, len(orders))
	for _, order := range orders {
		mergedOrders = append(mergedOrders, order)
	}
	mergedFundings := make([]types.FundingRequest, 0, len(fundings))
	for _, funding := range fundings {
		mergedFundings = append(mergedFundings, funding)
	}
	return mergedOrders, mergedFundings, nil
}
