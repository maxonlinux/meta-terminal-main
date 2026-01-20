package persistence

import (
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"time"

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
		WithSyncWrites(false).
		WithValueThreshold(256).
		WithNumCompactors(2)

	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}

	s := &Store{
		db:   db,
		path: path,
	}

	go s.backgroundSync(10 * time.Second)

	return s, nil
}

func (s *Store) backgroundSync(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		if atomic.LoadInt32(&s.closed) == 1 {
			return
		}
		s.db.Sync()
	}
}

func (s *Store) Close() error {
	if !atomic.CompareAndSwapInt32(&s.closed, 0, 1) {
		return nil
	}
	return s.db.Close()
}

func (s *Store) Compact() error {
	return s.db.RunValueLogGC(0.5)
}

func (s *Store) SaveOrder(order *types.Order) error {
	if atomic.LoadInt32(&s.closed) == 1 {
		return ErrStoreClosed
	}
	data := orderEncode(order)
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(orderKey(order.ID), data)
	})
}

func (s *Store) SaveBalance(balance *types.Balance) error {
	if atomic.LoadInt32(&s.closed) == 1 {
		return ErrStoreClosed
	}
	data := balanceEncode(balance)
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(balanceKey(balance.UserID, balance.Asset), data)
	})
}

func (s *Store) SavePosition(pos *types.Position) error {
	if atomic.LoadInt32(&s.closed) == 1 {
		return ErrStoreClosed
	}
	data := positionEncode(pos)
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(positionKey(pos.UserID, pos.Symbol), data)
	})
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
			order, err := orderDecode(data)
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
			balance, err := balanceDecode(data)
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
			pos, err := positionDecode(data)
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
		item, err := txn.Get(orderKey(id))
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
	return orderDecode(data)
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

func orderKey(id types.OrderID) []byte {
	return fmt.Appendf(nil, "o:%016x", id)
}

func balanceKey(userID types.UserID, asset string) []byte {
	return fmt.Appendf(nil, "b:%016x:%s", userID, asset)
}

func positionKey(userID types.UserID, symbol string) []byte {
	return fmt.Appendf(nil, "p:%016x:%s", userID, symbol)
}
