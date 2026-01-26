package persistence

import (
	"errors"
	"fmt"
	"os"
	"sync/atomic"

	"github.com/dgraph-io/badger/v3"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

var (
	ErrStoreClosed = errors.New("store is closed")
	ErrKeyNotFound = errors.New("key not found")
)

type Store struct {
	db     *badger.DB
	path   string
	closed int32
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return nil, err
	}

	opts := badger.DefaultOptions(path).
		WithLogger(nil).
		WithSyncWrites(false).
		WithValueThreshold(256).
		WithNumCompactors(4).
		WithNumLevelZeroTables(5).
		WithNumMemtables(8).
		WithMemTableSize(128 << 20).
		WithNumVersionsToKeep(0).
		WithBaseLevelSize(1024 << 20).
		WithLevelSizeMultiplier(10)

	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}

	return &Store{db: db, path: path}, nil
}

func (s *Store) DB() *badger.DB {
	if s == nil {
		return nil
	}
	return s.db
}

func (s *Store) Close() error {
	if !atomic.CompareAndSwapInt32(&s.closed, 0, 1) {
		return nil
	}
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func (s *Store) Compact() error {
	if s.db == nil {
		return ErrStoreClosed
	}
	return s.db.RunValueLogGC(0.5)
}

func (s *Store) SaveOrder(order *types.Order) error {
	if atomic.LoadInt32(&s.closed) == 1 {
		return ErrStoreClosed
	}
	data := EncodeOrder(order)
	return s.Set(OrderKey(order.ID), data)
}

func (s *Store) SaveBalance(balance *types.Balance) error {
	if atomic.LoadInt32(&s.closed) == 1 {
		return ErrStoreClosed
	}
	data := EncodeBalance(balance)
	return s.Set(BalanceKey(balance.UserID, balance.Asset), data)
}

func (s *Store) SavePosition(pos *types.Position) error {
	if atomic.LoadInt32(&s.closed) == 1 {
		return ErrStoreClosed
	}
	data := EncodePosition(pos)
	return s.Set(PositionKey(pos.UserID, pos.Symbol), data)
}

func (s *Store) LoadOrders(fn func(*types.Order) bool) error {
	if atomic.LoadInt32(&s.closed) == 1 {
		return ErrStoreClosed
	}
	prefix := []byte("o:")
	return s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			data, _ := it.Item().ValueCopy(nil)
			order, err := DecodeOrder(data)
			if err != nil {
				continue
			}
			if !fn(order) {
				break
			}
		}
		return nil
	})
}

func (s *Store) LoadBalances(fn func(*types.Balance) bool) error {
	if atomic.LoadInt32(&s.closed) == 1 {
		return ErrStoreClosed
	}
	prefix := []byte("b:")
	return s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			data, _ := it.Item().ValueCopy(nil)
			balance, err := DecodeBalance(data)
			if err != nil {
				continue
			}
			if !fn(balance) {
				break
			}
		}
		return nil
	})
}

func (s *Store) LoadPositions(fn func(*types.Position) bool) error {
	if atomic.LoadInt32(&s.closed) == 1 {
		return ErrStoreClosed
	}
	prefix := []byte("p:")
	return s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			data, _ := it.Item().ValueCopy(nil)
			pos, err := DecodePosition(data)
			if err != nil {
				continue
			}
			if !fn(pos) {
				break
			}
		}
		return nil
	})
}

func (s *Store) GetOrder(id types.OrderID) (*types.Order, error) {
	if atomic.LoadInt32(&s.closed) == 1 {
		return nil, ErrStoreClosed
	}
	var data []byte
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(OrderKey(id))
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return ErrKeyNotFound
			}
			return err
		}
		data, err = item.ValueCopy(nil)
		return err
	})
	if err != nil {
		return nil, err
	}
	return DecodeOrder(data)
}

func (s *Store) Set(key, value []byte) error {
	if atomic.LoadInt32(&s.closed) == 1 {
		return ErrStoreClosed
	}
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, value)
	})
}

func (s *Store) Get(key []byte) ([]byte, error) {
	if atomic.LoadInt32(&s.closed) == 1 {
		return nil, ErrStoreClosed
	}
	var data []byte
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return ErrKeyNotFound
			}
			return err
		}
		data, err = item.ValueCopy(nil)
		return err
	})
	return data, err
}

func (s *Store) WriteBatch(orders []*types.Order, balances []*types.Balance, positions []*types.Position) error {
	if atomic.LoadInt32(&s.closed) == 1 {
		return ErrStoreClosed
	}

	batch := s.db.NewWriteBatch()
	defer batch.Cancel()

	for _, order := range orders {
		if err := batch.Set(OrderKey(order.ID), EncodeOrder(order)); err != nil {
			return err
		}
	}

	for _, balance := range balances {
		if err := batch.Set(BalanceKey(balance.UserID, balance.Asset), EncodeBalance(balance)); err != nil {
			return err
		}
	}

	for _, position := range positions {
		if err := batch.Set(PositionKey(position.UserID, position.Symbol), EncodePosition(position)); err != nil {
			return err
		}
	}

	return batch.Flush()
}

func OrderKey(id types.OrderID) []byte {
	return fmt.Appendf(nil, "o:%016x", id)
}

func BalanceKey(userID types.UserID, asset string) []byte {
	return fmt.Appendf(nil, "b:%016x:%s", userID, asset)
}

func PositionKey(userID types.UserID, symbol string) []byte {
	return fmt.Appendf(nil, "p:%016x:%s", userID, symbol)
}
