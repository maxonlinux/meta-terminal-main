package snapshot

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

const (
	MagicNumber    = 0x534E4150
	Version        = 1
	SnapshotHeader = 16
)

type snapshotHeader struct {
	Magic     uint32
	Version   uint32
	Timestamp int64
	Offset    int64
	Checksum  uint32
}

type Snapshot struct {
	path    string
	maxSize int64
}

type SnapshotData struct {
	Version     uint32                `json:"version"`
	Timestamp   int64                 `json:"timestamp"`
	WALOffset   int64                 `json:"wal_offset"`
	Users       map[int64]*UserSnap   `json:"users"`
	Symbols     map[int32]*SymbolSnap `json:"symbols"`
	NextOrderID int64                 `json:"next_order_id"`
}

type UserSnap struct {
	Balances  map[string]*BalanceSnap `json:"balances"`
	Positions map[int32]*PositionSnap `json:"positions"`
}

type BalanceSnap struct {
	Asset     string `json:"asset"`
	Available int64  `json:"available"`
	Locked    int64  `json:"locked"`
	Margin    int64  `json:"margin"`
}

type PositionSnap struct {
	Symbol      int32 `json:"symbol"`
	Size        int64 `json:"size"`
	Side        int8  `json:"side"`
	EntryPrice  int64 `json:"entry_price"`
	Leverage    int8  `json:"leverage"`
	RealizedPnl int64 `json:"realized_pnl"`
}

type SymbolSnap struct {
	Category int8 `json:"category"`
}

func New(path string, maxSize int64) *Snapshot {
	_ = os.MkdirAll(path, 0755)
	return &Snapshot{
		path:    path,
		maxSize: maxSize,
	}
}

func (s *Snapshot) Create(st *state.State, walOffset int64) error {
	data := s.serialize(st, walOffset)

	dataBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot: %w", err)
	}

	checksum := s.checksum(dataBytes)

	header := snapshotHeader{
		Magic:     MagicNumber,
		Version:   Version,
		Timestamp: time.Now().Unix(),
		Offset:    walOffset,
		Checksum:  checksum,
	}

	timestamp := time.Now().UnixNano()
	filename := filepath.Join(s.path, fmt.Sprintf("snapshot_%d.snap", timestamp))
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to create snapshot file: %w", err)
	}
	defer func() { _ = file.Close() }()

	writer := bufio.NewWriter(file)

	if err := binary.Write(writer, binary.BigEndian, &header); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	if _, err := writer.Write(dataBytes); err != nil {
		return fmt.Errorf("failed to write data: %w", err)
	}

	_ = writer.Flush()
	_ = file.Close()

	s.rotateOldSnapshots()

	return nil
}

func (s *Snapshot) Load() (*state.State, int64, error) {
	filename, err := s.findLatest()
	if err != nil {
		return nil, 0, err
	}

	file, err := os.Open(filename)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to open snapshot: %w", err)
	}
	defer func() { _ = file.Close() }()

	var header snapshotHeader
	if err := binary.Read(file, binary.BigEndian, &header); err != nil {
		return nil, 0, fmt.Errorf("failed to read header: %w", err)
	}

	if header.Magic != MagicNumber {
		return nil, 0, fmt.Errorf("invalid magic number")
	}

	if header.Version != Version {
		return nil, 0, fmt.Errorf("invalid version")
	}

	fileInfo, _ := file.Stat()
	dataSize := fileInfo.Size() - SnapshotHeader
	if dataSize < 0 {
		dataSize = 0
	}

	dataBytes := make([]byte, dataSize)
	_, err = file.Read(dataBytes)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to read data: %w", err)
	}

	checksum := s.checksum(dataBytes)
	if checksum != header.Checksum {
		return nil, 0, fmt.Errorf("checksum mismatch: got %d, expected %d", checksum, header.Checksum)
	}

	for i := len(dataBytes) - 1; i >= 0; i-- {
		if dataBytes[i] != 0 {
			dataBytes = dataBytes[:i+1]
			break
		}
	}

	var data SnapshotData
	if err := json.Unmarshal(dataBytes, &data); err != nil {
		return nil, 0, fmt.Errorf("failed to unmarshal data: %w", err)
	}

	st := s.deserialize(&data)

	return st, header.Offset, nil
}

func (s *Snapshot) serialize(st *state.State, walOffset int64) *SnapshotData {
	data := &SnapshotData{
		Version:     Version,
		Timestamp:   time.Now().Unix(),
		WALOffset:   walOffset,
		Users:       make(map[int64]*UserSnap),
		Symbols:     make(map[int32]*SymbolSnap),
		NextOrderID: int64(st.NextOrderID),
	}

	for userID, us := range st.Users {
		userSnap := &UserSnap{
			Balances:  make(map[string]*BalanceSnap),
			Positions: make(map[int32]*PositionSnap),
		}

		for asset, bal := range us.Balances {
			userSnap.Balances[asset] = &BalanceSnap{
				Asset:     asset,
				Available: bal.Available,
				Locked:    bal.Locked,
				Margin:    bal.Margin,
			}
		}

		for symbol, pos := range us.Positions {
			userSnap.Positions[int32(symbol)] = &PositionSnap{
				Symbol:      int32(symbol),
				Size:        int64(pos.Size),
				Side:        pos.Side,
				EntryPrice:  int64(pos.EntryPrice),
				Leverage:    pos.Leverage,
				RealizedPnl: pos.RealizedPnl,
			}
		}

		data.Users[int64(userID)] = userSnap
	}

	for symbolID, ss := range st.Symbols {
		symbolSnap := &SymbolSnap{
			Category: ss.Category,
		}

		data.Symbols[int32(symbolID)] = symbolSnap
	}

	return data
}

func (s *Snapshot) deserialize(data *SnapshotData) *state.State {
	st := state.New()
	st.NextOrderID = types.OrderID(data.NextOrderID)

	for userID, userSnap := range data.Users {
		us := st.GetUserState(types.UserID(userID))

		for asset, balSnap := range userSnap.Balances {
			us.Balances[asset] = &types.UserBalance{
				UserID:    types.UserID(userID),
				Asset:     balSnap.Asset,
				Available: balSnap.Available,
				Locked:    balSnap.Locked,
				Margin:    balSnap.Margin,
			}
		}

		for symbol, posSnap := range userSnap.Positions {
			us.Positions[types.SymbolID(symbol)] = &types.Position{
				UserID:      types.UserID(userID),
				Symbol:      types.SymbolID(posSnap.Symbol),
				Size:        types.Quantity(posSnap.Size),
				Side:        posSnap.Side,
				EntryPrice:  types.Price(posSnap.EntryPrice),
				Leverage:    posSnap.Leverage,
				RealizedPnl: posSnap.RealizedPnl,
			}
		}
	}

	for symbolID, symbolSnap := range data.Symbols {
		ss := st.GetSymbolState(types.SymbolID(symbolID))
		ss.Category = symbolSnap.Category
	}

	return st
}

func (s *Snapshot) checksum(data []byte) uint32 {
	var sum uint32
	for _, b := range data {
		sum += uint32(b)
	}
	return sum
}

func (s *Snapshot) findLatest() (string, error) {
	entries, err := os.ReadDir(s.path)
	if err != nil {
		return "", fmt.Errorf("failed to read snapshot dir: %w", err)
	}

	var latest string
	var latestTime int64 = 0

	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".snap" {
			continue
		}

		filename := filepath.Join(s.path, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().UnixNano() > latestTime {
			latestTime = info.ModTime().UnixNano()
			latest = filename
		}
	}

	if latest == "" {
		return "", fmt.Errorf("no snapshot found")
	}

	return latest, nil
}

func (s *Snapshot) rotateOldSnapshots() {
	entries, err := os.ReadDir(s.path)
	if err != nil {
		return
	}

	var files []os.DirEntry
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".snap" {
			files = append(files, entry)
		}
	}

	if len(files) <= 5 {
		return
	}

	var oldest string
	var oldestTime int64 = 0

	for _, entry := range files {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		if oldestTime == 0 || info.ModTime().UnixNano() < oldestTime {
			oldestTime = info.ModTime().UnixNano()
			oldest = entry.Name()
		}
	}

	if oldest != "" {
		_ = os.Remove(filepath.Join(s.path, oldest))
	}
}
