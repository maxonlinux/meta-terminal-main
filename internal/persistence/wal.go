package persistence

import (
	"encoding/binary"
	"fmt"
	"os"
	"sync"

	"github.com/tidwall/wal"
)

type SnapshotStore struct {
	log        *wal.Log
	mu         sync.RWMutex
	cache      map[string][]byte
	closed     bool
	writeQueue chan *writeRequest
	done       chan struct{}
	wg         sync.WaitGroup
}

type writeRequest struct {
	service string
	shard   string
	data    []byte
	txId    uint64
	result  chan error
}

func New(path string) (*SnapshotStore, error) {
	os.MkdirAll(path, 0755)

	logFile, err := wal.Open(path+"/wal", &wal.Options{
		NoSync:      true,
		SegmentSize: 1024 * 1024,
	})
	if err != nil {
		return nil, fmt.Errorf("open wal: %w", err)
	}

	store := &SnapshotStore{
		log:        logFile,
		cache:      make(map[string][]byte),
		writeQueue: make(chan *writeRequest, 10000),
		done:       make(chan struct{}),
	}

	if firstIndex, err := logFile.FirstIndex(); err == nil && firstIndex > 0 {
		lastIndex, _ := logFile.LastIndex()
		for i := firstIndex; i <= lastIndex; i++ {
			if data, err := logFile.Read(i); err == nil {
				store.recover(i, data)
			}
		}
	}

	store.wg.Add(1)
	go store.processWrites()

	return store, nil
}

func (s *SnapshotStore) recover(index uint64, data []byte) {
	if len(data) < 4 {
		return
	}

	keyLen := binary.BigEndian.Uint32(data[:4])
	if int(keyLen)+4 > len(data) {
		return
	}

	key := string(data[4 : 4+keyLen])
	s.cache[key] = data[4+keyLen:]
}

func (s *SnapshotStore) processWrites() {
	defer s.wg.Done()

	for {
		select {
		case <-s.done:
			s.flushWrites()
			return
		case req := <-s.writeQueue:
			s.executeWrite(req)
		}
	}
}

func (s *SnapshotStore) flushWrites() {
	for {
		select {
		case req := <-s.writeQueue:
			s.executeWrite(req)
		default:
			return
		}
	}
}

func (s *SnapshotStore) executeWrite(req *writeRequest) {
	key := req.service + ":" + req.shard

	data := make([]byte, 4+len(key)+len(req.data))
	binary.BigEndian.PutUint32(data[:4], uint32(len(key)))
	copy(data[4:], key)
	copy(data[4+len(key):], req.data)

	err := s.log.Write(req.txId, data)

	if err == nil {
		s.mu.Lock()
		s.cache[key] = req.data
		s.mu.Unlock()
	}

	if req.result != nil {
		req.result <- err
	}
}

func (s *SnapshotStore) Save(service, shard string, data []byte, txId uint64) error {
	if s.closed {
		return fmt.Errorf("store is closed")
	}

	result := make(chan error, 1)

	select {
	case s.writeQueue <- &writeRequest{
		service: service,
		shard:   shard,
		data:    data,
		txId:    txId,
		result:  result,
	}:
		return <-result
	default:
		s.mu.Lock()
		defer s.mu.Unlock()

		key := service + ":" + shard
		writeData := make([]byte, 4+len(key)+len(data))
		binary.BigEndian.PutUint32(writeData[:4], uint32(len(key)))
		copy(writeData[4:], key)
		copy(writeData[4+len(key):], data)

		err := s.log.Write(txId, writeData)
		if err == nil {
			s.cache[key] = data
		}
		return err
	}
}

func (s *SnapshotStore) Load(service, shard string) ([]byte, bool) {
	key := service + ":" + shard

	s.mu.RLock()
	data, ok := s.cache[key]
	s.mu.RUnlock()
	return data, ok
}

func (s *SnapshotStore) SaveTx(txId uint64, service, status string) error {
	data := []byte(service + ":" + status)
	return s.log.Write(txId, data)
}

func (s *SnapshotStore) MarkCommitted(txId uint64) error {
	return s.log.Write(txId+0xFFFFFFFF, []byte("COMMITTED"))
}

func (s *SnapshotStore) GetSnapshot() map[string][]byte {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string][]byte)
	for k, v := range s.cache {
		result[k] = v
	}
	return result
}

func (s *SnapshotStore) PendingWrites() int {
	return len(s.writeQueue)
}

func (s *SnapshotStore) Close() error {
	s.closed = true
	close(s.done)
	s.wg.Wait()

	if s.log != nil {
		s.log.Sync()
		return s.log.Close()
	}
	return nil
}

func (s *SnapshotStore) Stats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return map[string]interface{}{
		"cache_size":  len(s.cache),
		"queue_depth": len(s.writeQueue),
	}
}

func (s *SnapshotStore) TruncateFront(index uint64) error {
	return s.log.TruncateFront(index)
}

func (s *SnapshotStore) TruncateBack(index uint64) error {
	return s.log.TruncateBack(index)
}
