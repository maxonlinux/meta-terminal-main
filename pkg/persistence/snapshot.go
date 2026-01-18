package persistence

import (
	"encoding/gob"
	"errors"
	"os"
	"path/filepath"

	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/maxonlinux/meta-terminal-go/pkg/utils"
)

// Snapshot captures a point-in-time view of engine state.
type Snapshot struct {
	LastSeq   uint64
	Created   uint64
	Orders    []types.Order
	Balances  []types.Balance
	Positions []types.Position
	Fundings  []types.FundingRequest
	Metadata  map[string]string
}

// SnapshotStore exposes the WAL sequence for snapshotting.
type SnapshotStore interface {
	Persistor
	LastSeq() uint64
}

// snapshotFilename stores the encoded snapshot payload.
const snapshotFilename = "snapshot.gob"

// WriteSnapshot writes a snapshot file to the provided directory.
func WriteSnapshot(path string, snapshot Snapshot) error {
	if path == "" {
		return errors.New("snapshot path is required")
	}
	if snapshot.Created == 0 {
		// Default timestamp keeps snapshot creation monotonic.
		snapshot.Created = utils.NowNano()
	}
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}

	file, err := os.Create(filepath.Join(path, snapshotFilename))
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := gob.NewEncoder(file)
	return encoder.Encode(snapshot)
}

// LoadSnapshotFile loads a snapshot from the provided directory.
func LoadSnapshotFile(path string) (Snapshot, error) {
	if path == "" {
		return Snapshot{}, errors.New("snapshot path is required")
	}
	file, err := os.Open(filepath.Join(path, snapshotFilename))
	if err != nil {
		return Snapshot{}, err
	}
	defer file.Close()

	var snapshot Snapshot
	decoder := gob.NewDecoder(file)
	if err := decoder.Decode(&snapshot); err != nil {
		return Snapshot{}, err
	}
	return snapshot, nil
}
