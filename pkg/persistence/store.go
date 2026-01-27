package persistence

import (
	"errors"
	"fmt"
	"os"
	"sync/atomic"

	"github.com/dgraph-io/badger/v3"
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

func (s *Store) IterateEvents(fn func(key, value []byte) bool) error {
	if atomic.LoadInt32(&s.closed) == 1 {
		return ErrStoreClosed
	}
	prefix := []byte("e:")
	return s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			key := item.KeyCopy(nil)
			value, _ := item.ValueCopy(nil)
			if !fn(key, value) {
				break
			}
		}
		return nil
	})
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

func EventKey(seq uint64) []byte {
	return fmt.Appendf(nil, "e:%020d", seq)
}
