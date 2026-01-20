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

var (
	ErrKeyNotFound  = errors.New("key not found")
	ErrStoreClosed  = errors.New("store is closed")
	ErrTxInProgress = errors.New("transaction already in progress")
	ErrNoTx         = errors.New("no transaction in progress")
)

type PebbleKV struct {
	db       *pebble.DB
	path     string
	closed   int32
	txBatch  *pebble.Batch
	txActive int32
}

func Open(path string) (*PebbleKV, error) {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return nil, err
	}

	db, err := pebble.Open(path, &pebble.Options{
		Cache:        pebble.NewCache(64 << 20),
		MaxOpenFiles: 1000,
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

func (s *PebbleKV) Set(key, value []byte) error {
	if atomic.LoadInt32(&s.closed) == 1 {
		return ErrStoreClosed
	}
	if s.txBatch != nil {
		s.txBatch.Set(key, value, nil)
		return nil
	}
	return s.db.Set(key, value, pebble.Sync)
}

func (s *PebbleKV) Get(key []byte) ([]byte, error) {
	if atomic.LoadInt32(&s.closed) == 1 {
		return nil, ErrStoreClosed
	}
	data, closer, err := s.db.Get(key)
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return nil, ErrKeyNotFound
		}
		return nil, err
	}
	defer closer.Close()
	return data, nil
}

func (s *PebbleKV) Delete(key []byte) error {
	if atomic.LoadInt32(&s.closed) == 1 {
		return ErrStoreClosed
	}
	if s.txBatch != nil {
		s.txBatch.Delete(key, nil)
		return nil
	}
	return s.db.Delete(key, pebble.Sync)
}

func (s *PebbleKV) Begin() {
	if atomic.CompareAndSwapInt32(&s.txActive, 0, 1) {
		s.txBatch = s.db.NewBatch()
	}
}

func (s *PebbleKV) HasActiveTx() bool {
	return atomic.LoadInt32(&s.txActive) == 1
}

func (s *PebbleKV) Rollback() {
	if atomic.CompareAndSwapInt32(&s.txActive, 1, 0) {
		s.txBatch = nil
	}
}

func (s *PebbleKV) Commit() error {
	if !atomic.CompareAndSwapInt32(&s.txActive, 1, 0) {
		return ErrNoTx
	}
	defer func() { s.txBatch = nil }()
	return s.txBatch.Commit(pebble.Sync)
}

func (s *PebbleKV) PutOrder(order *types.Order) error {
	if atomic.LoadInt32(&s.closed) == 1 {
		return ErrStoreClosed
	}
	data, err := encodeOrder(order)
	if err != nil {
		return err
	}
	if s.txBatch != nil {
		s.txBatch.Set(orderKey(order.ID), data, nil)
		return nil
	}
	return s.db.Set(orderKey(order.ID), data, pebble.Sync)
}

func (s *PebbleKV) DeleteOrder(id types.OrderID) error {
	if atomic.LoadInt32(&s.closed) == 1 {
		return ErrStoreClosed
	}
	if s.txBatch != nil {
		s.txBatch.Delete(orderKey(id), nil)
		return nil
	}
	return s.db.Delete(orderKey(id), pebble.Sync)
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

func (s *PebbleKV) PutBalance(balance *types.Balance) error {
	if atomic.LoadInt32(&s.closed) == 1 {
		return ErrStoreClosed
	}
	data, err := encodeBalance(balance)
	if err != nil {
		return err
	}
	if s.txBatch != nil {
		s.txBatch.Set(balanceKey(balance.UserID, balance.Asset), data, nil)
		return nil
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
	if s.txBatch != nil {
		s.txBatch.Set(positionKey(pos.UserID, pos.Symbol), data, nil)
		return nil
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

func (s *PebbleKV) DeletePosition(userID types.UserID, symbol string) error {
	if atomic.LoadInt32(&s.closed) == 1 {
		return ErrStoreClosed
	}
	if s.txBatch != nil {
		s.txBatch.Delete(positionKey(userID, symbol), nil)
		return nil
	}
	return s.db.Delete(positionKey(userID, symbol), pebble.Sync)
}

func (s *PebbleKV) PutReduceOnly(symbol string, orderID types.OrderID, exposure []byte) error {
	if atomic.LoadInt32(&s.closed) == 1 {
		return ErrStoreClosed
	}
	key := reduceOnlyKey(symbol, orderID)
	if s.txBatch != nil {
		s.txBatch.Set(key, exposure, nil)
		return nil
	}
	return s.db.Set(key, exposure, pebble.Sync)
}

func (s *PebbleKV) DeleteReduceOnly(symbol string, orderID types.OrderID) error {
	if atomic.LoadInt32(&s.closed) == 1 {
		return ErrStoreClosed
	}
	key := reduceOnlyKey(symbol, orderID)
	if s.txBatch != nil {
		s.txBatch.Delete(key, nil)
		return nil
	}
	return s.db.Delete(key, pebble.Sync)
}

func (s *PebbleKV) RangeReduceOnly(symbol string, fn func(orderID types.OrderID, exposure []byte) bool) error {
	if atomic.LoadInt32(&s.closed) == 1 {
		return ErrStoreClosed
	}
	prefix := reduceOnlyPrefix(symbol)
	iter, err := s.db.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: append(append([]byte{}, prefix...), 0xff),
	})
	if err != nil {
		return err
	}
	defer iter.Close()
	for iter.First(); iter.Valid(); iter.Next() {
		id := extractReduceOnlyOrderID(iter.Key())
		if !fn(id, iter.Value()) {
			break
		}
	}
	return nil
}

func (s *PebbleKV) PutTrigger(symbol string, orderID types.OrderID, orderData []byte) error {
	if atomic.LoadInt32(&s.closed) == 1 {
		return ErrStoreClosed
	}
	key := triggerKey(symbol, orderID)
	if s.txBatch != nil {
		s.txBatch.Set(key, orderData, nil)
		return nil
	}
	return s.db.Set(key, orderData, pebble.Sync)
}

func (s *PebbleKV) DeleteTrigger(symbol string, orderID types.OrderID) error {
	if atomic.LoadInt32(&s.closed) == 1 {
		return ErrStoreClosed
	}
	key := triggerKey(symbol, orderID)
	if s.txBatch != nil {
		s.txBatch.Delete(key, nil)
		return nil
	}
	return s.db.Delete(key, pebble.Sync)
}

func (s *PebbleKV) RangeTriggers(symbol string, fn func(orderID types.OrderID, data []byte) bool) error {
	if atomic.LoadInt32(&s.closed) == 1 {
		return ErrStoreClosed
	}
	prefix := triggerPrefix(symbol)
	iter, err := s.db.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: append(append([]byte{}, prefix...), 0xff),
	})
	if err != nil {
		return err
	}
	defer iter.Close()
	for iter.First(); iter.Valid(); iter.Next() {
		id := extractTriggerOrderID(iter.Key())
		if !fn(id, iter.Value()) {
			break
		}
	}
	return nil
}

func (s *PebbleKV) SetMeta(key string, value uint64) error {
	if atomic.LoadInt32(&s.closed) == 1 {
		return ErrStoreClosed
	}
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, value)
	if s.txBatch != nil {
		s.txBatch.Set(metaKey(key), buf, nil)
		return nil
	}
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
	if atomic.LoadInt32(&s.closed) == 1 {
		return ErrStoreClosed
	}
	return s.scanKeys(orderPrefix(), func(key, value []byte) bool {
		order, err := decodeOrder(value)
		if err != nil {
			return true
		}
		return fn(order)
	})
}

func (s *PebbleKV) RangeBalances(fn func(balance *types.Balance) bool) error {
	if atomic.LoadInt32(&s.closed) == 1 {
		return ErrStoreClosed
	}
	return s.scanKeys(balancePrefix(), func(key, value []byte) bool {
		balance, err := decodeBalance(value)
		if err != nil {
			return true
		}
		return fn(balance)
	})
}

func (s *PebbleKV) RangePositions(fn func(pos *types.Position) bool) error {
	if atomic.LoadInt32(&s.closed) == 1 {
		return ErrStoreClosed
	}
	return s.scanKeys(positionPrefix(), func(key, value []byte) bool {
		pos, err := decodePosition(value)
		if err != nil {
			return true
		}
		return fn(pos)
	})
}

func (s *PebbleKV) scanKeys(prefix []byte, fn func(key, value []byte) bool) error {
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

func reduceOnlyKey(symbol string, orderID types.OrderID) []byte {
	return []byte(fmt.Sprintf("ro:%s:%016x", symbol, orderID))
}

func reduceOnlyPrefix(symbol string) []byte {
	return []byte(fmt.Sprintf("ro:%s:", symbol))
}

func extractReduceOnlyOrderID(key []byte) types.OrderID {
	var id int64
	fmt.Sscanf(string(key), "ro:%*s:%016x", &id)
	return types.OrderID(id)
}

func triggerKey(symbol string, orderID types.OrderID) []byte {
	return []byte(fmt.Sprintf("tr:%s:%016x", symbol, orderID))
}

func triggerPrefix(symbol string) []byte {
	return []byte(fmt.Sprintf("tr:%s:", symbol))
}

func extractTriggerOrderID(key []byte) types.OrderID {
	var id int64
	fmt.Sscanf(string(key), "tr:%*s:%016x", &id)
	return types.OrderID(id)
}

func metaKey(key string) []byte {
	return []byte(fmt.Sprintf("m:%s", key))
}

func encodeOrder(order *types.Order) ([]byte, error) {
	buf := bytes.Buffer{}
	if err := gob.NewEncoder(&buf).Encode(order); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decodeOrder(data []byte) (*types.Order, error) {
	var order types.Order
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&order); err != nil {
		return nil, err
	}
	return &order, nil
}

func encodeBalance(balance *types.Balance) ([]byte, error) {
	buf := bytes.Buffer{}
	if err := gob.NewEncoder(&buf).Encode(balance); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decodeBalance(data []byte) (*types.Balance, error) {
	var balance types.Balance
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&balance); err != nil {
		return nil, err
	}
	return &balance, nil
}

func encodePosition(pos *types.Position) ([]byte, error) {
	buf := bytes.Buffer{}
	if err := gob.NewEncoder(&buf).Encode(pos); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decodePosition(data []byte) (*types.Position, error) {
	var pos types.Position
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&pos); err != nil {
		return nil, err
	}
	return &pos, nil
}
