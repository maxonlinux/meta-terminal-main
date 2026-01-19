package persistence

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"

	"github.com/cockroachdb/pebble"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

const (
	cfOrders    = "orders"
	cfBalances  = "balances"
	cfPositions = "positions"
	cfFundings  = "fundings"
	cfMeta      = "meta"
)

var (
	ErrKeyNotFound = errors.New("key not found")
	ErrStoreClosed = errors.New("store is closed")
)

type PebbleKV struct {
	db     *pebble.DB
	path   string
	closed int32
}

func OpenPebbleKV(path string) (*PebbleKV, error) {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return nil, err
	}

	db, err := pebble.Open(path, &pebble.Options{
		// Use default options, can be tuned later
	})
	if err != nil {
		return nil, err
	}

	return &PebbleKV{
		db:   db,
		path: path,
	}, nil
}

func (s *PebbleKV) Close() error {
	if !atomic.CompareAndSwapInt32(&s.closed, 0, 1) {
		return nil
	}
	return s.db.Close()
}

func (s *PebbleKV) Compact() error {
	return s.db.Compact(nil, nil, false)
}

func (s *PebbleKV) Checkpoint() error {
	checkpointDir := filepath.Join(s.path, "checkpoint")
	return s.db.Checkpoint(checkpointDir)
}

func (s *PebbleKV) GetDB() *pebble.DB {
	return s.db
}

func (s *PebbleKV) PutOrder(order *types.Order) error {
	if atomic.LoadInt32(&s.closed) == 1 {
		return ErrStoreClosed
	}
	data, err := encodeOrder(order)
	if err != nil {
		return err
	}
	return s.db.Set(orderKey(order.ID), data, pebble.Sync)
}

func (s *PebbleKV) GetOrder(id types.OrderID) (*types.Order, error) {
	if atomic.LoadInt32(&s.closed) == 1 {
		return nil, ErrStoreClosed
	}
	data, closer, err := s.db.Get(orderKey(id))
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return nil, ErrKeyNotFound
		}
		return nil, err
	}
	defer closer.Close()
	return decodeOrder(data)
}

func (s *PebbleKV) DeleteOrder(id types.OrderID) error {
	if atomic.LoadInt32(&s.closed) == 1 {
		return ErrStoreClosed
	}
	return s.db.Delete(orderKey(id), pebble.Sync)
}

func (s *PebbleKV) PutBalance(balance *types.Balance) error {
	if atomic.LoadInt32(&s.closed) == 1 {
		return ErrStoreClosed
	}
	data, err := encodeBalance(balance)
	if err != nil {
		return err
	}
	return s.db.Set(balanceKey(balance.UserID, balance.Asset), data, pebble.Sync)
}

func (s *PebbleKV) GetBalance(userID types.UserID, asset string) (*types.Balance, error) {
	if atomic.LoadInt32(&s.closed) == 1 {
		return nil, ErrStoreClosed
	}
	data, closer, err := s.db.Get(balanceKey(userID, asset))
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return nil, ErrKeyNotFound
		}
		return nil, err
	}
	defer closer.Close()
	return decodeBalance(data)
}

func (s *PebbleKV) PutPosition(pos *types.Position) error {
	if atomic.LoadInt32(&s.closed) == 1 {
		return ErrStoreClosed
	}
	data, err := encodePosition(pos)
	if err != nil {
		return err
	}
	return s.db.Set(positionKey(pos.UserID, pos.Symbol), data, pebble.Sync)
}

func (s *PebbleKV) GetPosition(userID types.UserID, symbol string) (*types.Position, error) {
	if atomic.LoadInt32(&s.closed) == 1 {
		return nil, ErrStoreClosed
	}
	data, closer, err := s.db.Get(positionKey(userID, symbol))
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return nil, ErrKeyNotFound
		}
		return nil, err
	}
	defer closer.Close()
	return decodePosition(data)
}

func (s *PebbleKV) PutFunding(req *types.FundingRequest) error {
	if atomic.LoadInt32(&s.closed) == 1 {
		return ErrStoreClosed
	}
	data, err := encodeFunding(req)
	if err != nil {
		return err
	}
	return s.db.Set(fundingKey(req.ID), data, pebble.Sync)
}

func (s *PebbleKV) GetFunding(id types.FundingID) (*types.FundingRequest, error) {
	if atomic.LoadInt32(&s.closed) == 1 {
		return nil, ErrStoreClosed
	}
	data, closer, err := s.db.Get(fundingKey(id))
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return nil, ErrKeyNotFound
		}
		return nil, err
	}
	defer closer.Close()
	return decodeFunding(data)
}

func (s *PebbleKV) SetMeta(key string, value uint64) error {
	if atomic.LoadInt32(&s.closed) == 1 {
		return ErrStoreClosed
	}
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, value)
	return s.db.Set(metaKey(key), buf, pebble.Sync)
}

func (s *PebbleKV) GetMeta(key string) (uint64, error) {
	if atomic.LoadInt32(&s.closed) == 1 {
		return 0, ErrStoreClosed
	}
	data, closer, err := s.db.Get(metaKey(key))
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return 0, nil
		}
		return 0, err
	}
	defer closer.Close()
	return binary.LittleEndian.Uint64(data), nil
}

func (s *PebbleKV) RangeOrders(fn func(order *types.Order) bool) error {
	return s.rangeKeys(orderPrefix(), func(key, value []byte) bool {
		order, err := decodeOrder(value)
		if err != nil {
			return true
		}
		return fn(order)
	})
}

func (s *PebbleKV) RangeBalances(fn func(balance *types.Balance) bool) error {
	return s.rangeKeys(balancePrefix(), func(key, value []byte) bool {
		balance, err := decodeBalance(value)
		if err != nil {
			return true
		}
		return fn(balance)
	})
}

func (s *PebbleKV) RangePositions(fn func(pos *types.Position) bool) error {
	return s.rangeKeys(positionPrefix(), func(key, value []byte) bool {
		pos, err := decodePosition(value)
		if err != nil {
			return true
		}
		return fn(pos)
	})
}

func (s *PebbleKV) RangeFundings(fn func(req *types.FundingRequest) bool) error {
	return s.rangeKeys(fundingPrefix(), func(key, value []byte) bool {
		req, err := decodeFunding(value)
		if err != nil {
			return true
		}
		return fn(req)
	})
}

func (s *PebbleKV) rangeKeys(prefix []byte, fn func(key, value []byte) bool) error {
	if atomic.LoadInt32(&s.closed) == 1 {
		return ErrStoreClosed
	}
	iter, err := s.db.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: append(append([]byte{}, prefix...), 0xff),
	})
	if err != nil {
		return err
	}
	defer iter.Close()

	for iter.First(); iter.Valid(); iter.Next() {
		if !fn(iter.Key(), iter.Value()) {
			break
		}
	}
	return nil
}

func (s *PebbleKV) BatchWrite(ops []BatchOp) error {
	if atomic.LoadInt32(&s.closed) == 1 {
		return ErrStoreClosed
	}
	batch := s.db.NewBatch()
	for _, op := range ops {
		switch op.Kind {
		case OpPutOrder:
			data, _ := encodeOrder(op.Order)
			batch.Set(orderKey(op.Order.ID), data, nil)
		case OpDeleteOrder:
			batch.Delete(orderKey(op.Order.ID), nil)
		case OpPutBalance:
			data, _ := encodeBalance(op.Balance)
			batch.Set(balanceKey(op.Balance.UserID, op.Balance.Asset), data, nil)
		case OpPutPosition:
			data, _ := encodePosition(op.Position)
			batch.Set(positionKey(op.Position.UserID, op.Position.Symbol), data, nil)
		case OpPutFunding:
			data, _ := encodeFunding(op.Funding)
			batch.Set(fundingKey(op.Funding.ID), data, nil)
		}
	}
	return batch.Commit(pebble.Sync)
}

type BatchOp struct {
	Kind     BatchOpKind
	Order    *types.Order
	Balance  *types.Balance
	Position *types.Position
	Funding  *types.FundingRequest
}

type BatchOpKind int

const (
	OpPutOrder BatchOpKind = iota
	OpDeleteOrder
	OpPutBalance
	OpPutPosition
	OpPutFunding
)

func orderKey(id types.OrderID) []byte {
	return []byte(fmt.Sprintf("o:%016x", id))
}

func orderPrefix() []byte {
	return []byte("o:")
}

func balanceKey(userID types.UserID, asset string) []byte {
	return []byte(fmt.Sprintf("b:%016x:%s", userID, asset))
}

func balancePrefix() []byte {
	return []byte("b:")
}

func positionKey(userID types.UserID, symbol string) []byte {
	return []byte(fmt.Sprintf("p:%016x:%s", userID, symbol))
}

func positionPrefix() []byte {
	return []byte("p:")
}

func fundingKey(id types.FundingID) []byte {
	return []byte(fmt.Sprintf("f:%016x", id))
}

func fundingPrefix() []byte {
	return []byte("f:")
}

func metaKey(key string) []byte {
	return []byte(fmt.Sprintf("m:%s", key))
}

func encodeOrder(order *types.Order) ([]byte, error) {
	buf := bytes.Buffer{}
	encoder := gob.NewEncoder(&buf)
	if err := encoder.Encode(order); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decodeOrder(data []byte) (*types.Order, error) {
	var order types.Order
	decoder := gob.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&order); err != nil {
		return nil, err
	}
	return &order, nil
}

func encodeBalance(balance *types.Balance) ([]byte, error) {
	buf := bytes.Buffer{}
	encoder := gob.NewEncoder(&buf)
	if err := encoder.Encode(balance); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decodeBalance(data []byte) (*types.Balance, error) {
	var balance types.Balance
	decoder := gob.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&balance); err != nil {
		return nil, err
	}
	return &balance, nil
}

func encodePosition(pos *types.Position) ([]byte, error) {
	buf := bytes.Buffer{}
	encoder := gob.NewEncoder(&buf)
	if err := encoder.Encode(pos); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decodePosition(data []byte) (*types.Position, error) {
	var pos types.Position
	decoder := gob.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&pos); err != nil {
		return nil, err
	}
	return &pos, nil
}

func encodeFunding(req *types.FundingRequest) ([]byte, error) {
	buf := bytes.Buffer{}
	encoder := gob.NewEncoder(&buf)
	if err := encoder.Encode(req); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decodeFunding(data []byte) (*types.FundingRequest, error) {
	var req types.FundingRequest
	decoder := gob.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&req); err != nil {
		return nil, err
	}
	return &req, nil
}
