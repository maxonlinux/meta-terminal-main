package persistence

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"errors"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/maxonlinux/meta-terminal-go/pkg/config"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/maxonlinux/meta-terminal-go/pkg/utils"
)

// EventKind describes the payload kind stored in persistence.
type EventKind uint8

const (
	EventOrder   EventKind = 1
	EventTrade   EventKind = 2
	EventFunding EventKind = 3
)

// Record is a decoded persistence event.
type Record struct {
	Kind    EventKind
	Order   *OrderEvent
	Trade   *TradeEvent
	Funding *FundingEvent
}

// Handler consumes decoded persistence records.
type Handler func(record Record) error

// ReplaySource provides ordered events for replay.
type ReplaySource interface {
	Replay(fromSeq uint64, handler Handler) error
}

// Replay replays records from a source starting at fromSeq.
func Replay(source ReplaySource, fromSeq uint64, handler Handler) error {
	if source == nil {
		return errors.New("replay source is required")
	}
	if handler == nil {
		return errors.New("replay handler is required")
	}
	// Default to the first sequence when not specified.
	if fromSeq == 0 {
		fromSeq = 1
	}
	return source.Replay(fromSeq, handler)
}

// ActionType describes the kind of order event persisted to disk.
type ActionType uint8

const (
	// ActionOrderCreated persists an order creation event.
	ActionOrderCreated ActionType = iota + 1
	// ActionOrderAmended persists an order amendment event.
	ActionOrderAmended
	// ActionOrderCanceled persists a cancellation event.
	ActionOrderCanceled
	// ActionOrderUpdated persists order state updates after matching.
	ActionOrderUpdated
)

// Config tunes the WAL batcher behavior for durability and throughput.
type Config struct {
	BatchInterval time.Duration
	BatchMaxItems int
	BatchMaxBytes int
	QueueSize     int
}

// DefaultConfig returns the default WAL tuning values.
func DefaultConfig() Config {
	cfg := config.Load().WAL
	return Config{
		BatchInterval: cfg.BatchInterval,
		BatchMaxItems: cfg.BatchMaxItems,
		BatchMaxBytes: cfg.BatchMaxBytes,
		QueueSize:     cfg.QueueSize,
	}
}

// ErrPersistClosed is returned when the persistor is already closed.
var ErrPersistClosed = errors.New("persistence store is closed")

// OrderEvent captures an order lifecycle event for recovery.
type OrderEvent struct {
	Seq       uint64
	Action    ActionType
	Timestamp uint64
	Order     types.Order
	NewQty    types.Quantity
}

// TradeEvent captures a trade execution for recovery.
type TradeEvent struct {
	Seq       uint64
	Timestamp uint64
	Trade     types.Trade
}

// FundingEvent captures funding state transitions for recovery.
type FundingEvent struct {
	Seq       uint64
	Timestamp uint64
	Funding   types.FundingRequest
}

// Persistor defines the persistence API used by the engine.
type Persistor interface {
	AppendOrderCreated(order *types.Order) error
	AppendOrderAmended(order *types.Order, newQty types.Quantity) error
	AppendOrderCanceled(order *types.Order) error
	AppendOrderUpdated(order *types.Order) error
	AppendTrade(trade types.Trade) error
	AppendFunding(funding types.FundingRequest) error
	Close() error
}

type walItem struct {
	kind    uint8
	order   *OrderEvent
	trade   *TradeEvent
	funding *FundingEvent
}

// Store appends order and trade events to a custom WAL.
type Store struct {
	file *os.File
	seq  uint64
	cfg  Config

	items   chan walItem
	flushCh chan chan error
	done    chan struct{}
	wg      sync.WaitGroup

	closed int32
}

// OpenStore opens a custom WAL-backed store at the provided path.
func OpenStore(path string) (*Store, error) {
	return OpenStoreWithConfig(path, DefaultConfig())
}

// OpenStoreWithConfig opens a custom WAL-backed store with explicit config.
func OpenStoreWithConfig(path string, cfg Config) (*Store, error) {
	// WAL path is required to create the store.
	if path == "" {
		return nil, errors.New("wal path is required")
	}
	if err := os.MkdirAll(path, 0o700); err != nil {
		return nil, err
	}

	walPath := filepath.Join(path, "events.wal")
	file, err := os.OpenFile(walPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}

	queueSize := cfg.QueueSize
	if queueSize <= 0 {
		queueSize = DefaultConfig().QueueSize
	}

	store := &Store{
		file:    file,
		cfg:     cfg,
		items:   make(chan walItem, queueSize),
		flushCh: make(chan chan error, 1),
		done:    make(chan struct{}),
	}
	store.wg.Add(1)
	go store.runWriter()
	return store, nil
}

// AppendOrderCreated persists a newly created order.
func (s *Store) AppendOrderCreated(order *types.Order) error {
	return s.enqueueOrder(ActionOrderCreated, order, types.Quantity{})
}

// AppendOrderAmended persists an amendment event.
func (s *Store) AppendOrderAmended(order *types.Order, newQty types.Quantity) error {
	return s.enqueueOrder(ActionOrderAmended, order, newQty)
}

// AppendOrderCanceled persists a cancellation event.
func (s *Store) AppendOrderCanceled(order *types.Order) error {
	return s.enqueueOrder(ActionOrderCanceled, order, types.Quantity{})
}

// AppendOrderUpdated persists a post-match order update.
func (s *Store) AppendOrderUpdated(order *types.Order) error {
	return s.enqueueOrder(ActionOrderUpdated, order, types.Quantity{})
}

// AppendTrade persists a trade execution event.
func (s *Store) AppendTrade(trade types.Trade) error {
	event := TradeEvent{
		Seq:       s.nextSeq(),
		Timestamp: utils.NowNano(),
		Trade:     trade,
	}
	return s.enqueue(walItem{kind: uint8(EventTrade), trade: &event})
}

// AppendFunding persists a funding lifecycle event.
func (s *Store) AppendFunding(funding types.FundingRequest) error {
	event := FundingEvent{
		Seq:       s.nextSeq(),
		Timestamp: utils.NowNano(),
		Funding:   funding,
	}
	return s.enqueue(walItem{kind: uint8(EventFunding), funding: &event})
}

// Close flushes the WAL and closes the underlying file.
func (s *Store) Close() error {
	if !atomic.CompareAndSwapInt32(&s.closed, 0, 1) {
		return nil
	}
	close(s.done)
	s.wg.Wait()
	return s.file.Close()
}

// Flush forces all pending records to be written and synced.
func (s *Store) Flush() error {
	if atomic.LoadInt32(&s.closed) == 1 {
		return ErrPersistClosed
	}
	// Wait for the writer to flush pending batches.
	resp := make(chan error, 1)
	select {
	case s.flushCh <- resp:
		return <-resp
	case <-s.done:
		return ErrPersistClosed
	}
}

// LastSeq reports the latest WAL sequence number.
func (s *Store) LastSeq() uint64 {
	return atomic.LoadUint64(&s.seq)
}

func (s *Store) enqueueOrder(action ActionType, order *types.Order, newQty types.Quantity) error {
	event := OrderEvent{
		Seq:       s.nextSeq(),
		Action:    action,
		Timestamp: utils.NowNano(),
		Order:     *order,
		NewQty:    newQty,
	}
	return s.enqueue(walItem{kind: uint8(EventOrder), order: &event})
}

func (s *Store) enqueue(item walItem) error {
	if atomic.LoadInt32(&s.closed) == 1 {
		return ErrPersistClosed
	}
	// Ignore empty items rather than blocking the writer.
	if item.kind == 0 {
		return nil
	}
	select {
	case s.items <- item:
		return nil
	case <-s.done:
		return ErrPersistClosed
	}
}

func (s *Store) runWriter() {
	defer s.wg.Done()

	interval := s.cfg.BatchInterval
	if interval <= 0 {
		interval = DefaultConfig().BatchInterval
	}
	maxItems := s.cfg.BatchMaxItems
	if maxItems <= 0 {
		maxItems = DefaultConfig().BatchMaxItems
	}
	maxBytes := s.cfg.BatchMaxBytes
	if maxBytes <= 0 {
		maxBytes = DefaultConfig().BatchMaxBytes
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	buf := bytes.NewBuffer(make([]byte, 0, maxBytes))
	batchCount := 0

	flush := func() error {
		if batchCount == 0 {
			return nil
		}
		if _, err := s.file.Write(buf.Bytes()); err != nil {
			return err
		}
		if err := s.file.Sync(); err != nil {
			return err
		}
		buf.Reset()
		batchCount = 0
		return nil
	}

	for {
		select {
		case item := <-s.items:
			encodeRecord(buf, item)
			batchCount++
			if batchCount >= maxItems || buf.Len() >= maxBytes {
				_ = flush()
			}
		case req := <-s.flushCh:
			for {
				select {
				case pending := <-s.items:
					encodeRecord(buf, pending)
					batchCount++
					if batchCount >= maxItems || buf.Len() >= maxBytes {
						_ = flush()
					}
				default:
					req <- flush()
					req = nil
				}
				if req == nil {
					break
				}
			}
		case <-ticker.C:
			_ = flush()
		case <-s.done:
			_ = flush()
			return
		}
	}
}

func (s *Store) nextSeq() uint64 {
	return atomic.AddUint64(&s.seq, 1)
}

func encodeRecord(buf *bytes.Buffer, item walItem) {
	payload := bytes.Buffer{}
	encoder := gob.NewEncoder(&payload)
	switch item.kind {
	case uint8(EventOrder):
		_ = encoder.Encode(item.order)
	case uint8(EventTrade):
		_ = encoder.Encode(item.trade)
	case uint8(EventFunding):
		_ = encoder.Encode(item.funding)
	default:
		return
	}

	length := uint32(payload.Len())
	crc := crc32.ChecksumIEEE(append([]byte{item.kind}, payload.Bytes()...))
	// WAL record layout: length, kind, payload, crc.
	_ = binary.Write(buf, binary.LittleEndian, length)
	_ = buf.WriteByte(item.kind)
	_, _ = buf.Write(payload.Bytes())
	_ = binary.Write(buf, binary.LittleEndian, crc)
}

// ReplayWAL replays WAL records from the provided directory.
func ReplayWAL(path string, fromSeq uint64, handler Handler) (uint64, error) {
	if path == "" {
		return 0, errors.New("wal path is required")
	}
	if handler == nil {
		return 0, errors.New("replay handler is required")
	}
	// Default to the first WAL sequence when not provided.
	if fromSeq == 0 {
		fromSeq = 1
	}

	file, err := os.Open(filepath.Join(path, "events.wal"))
	if err != nil {
		return 0, err
	}
	defer file.Close()

	var lastSeq uint64
	for {
		record, kind, err := readWALRecord(file)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return lastSeq, err
		}
		var eventSeq uint64
		var payload Record
		var decodeErr error

		switch kind {
		case uint8(EventOrder):
			var event OrderEvent
			decodeErr = decodeWAL(record, &event)
			payload = Record{Kind: EventOrder, Order: &event}
			eventSeq = event.Seq
		case uint8(EventTrade):
			var event TradeEvent
			decodeErr = decodeWAL(record, &event)
			payload = Record{Kind: EventTrade, Trade: &event}
			eventSeq = event.Seq
		case uint8(EventFunding):
			var event FundingEvent
			decodeErr = decodeWAL(record, &event)
			payload = Record{Kind: EventFunding, Funding: &event}
			eventSeq = event.Seq
		default:
			continue
		}

		if decodeErr != nil {
			return lastSeq, decodeErr
		}
		if eventSeq > lastSeq {
			lastSeq = eventSeq
		}
		if eventSeq >= fromSeq {
			if handlerErr := handler(payload); handlerErr != nil {
				return lastSeq, handlerErr
			}
		}
	}

	return lastSeq, nil
}

func readWALRecord(r io.Reader) ([]byte, uint8, error) {
	var length uint32
	if err := binary.Read(r, binary.LittleEndian, &length); err != nil {
		return nil, 0, err
	}
	kind := make([]byte, 1)
	if _, err := io.ReadFull(r, kind); err != nil {
		return nil, 0, err
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, 0, err
	}
	var crc uint32
	if err := binary.Read(r, binary.LittleEndian, &crc); err != nil {
		return nil, 0, err
	}
	checksum := crc32.ChecksumIEEE(append(kind, payload...))
	if checksum != crc {
		return nil, 0, errors.New("wal checksum mismatch")
	}
	// Payload is already validated with crc.
	return payload, kind[0], nil
}

func decodeWAL(data []byte, target any) error {
	decoder := gob.NewDecoder(bytes.NewReader(data))
	return decoder.Decode(target)
}
