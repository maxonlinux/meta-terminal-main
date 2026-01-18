package persistence

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/maxonlinux/meta-terminal-go/pkg/config"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/maxonlinux/meta-terminal-go/pkg/utils"
	"github.com/nats-io/nats.go"
)

// NATSConfig configures the JetStream persistence backend.
type NATSConfig struct {
	URL               string
	Stream            string
	EventSubject      string
	OrderSubject      string
	TradeSubject      string
	AsyncMaxPending   int
	PublishAckTimeout time.Duration
	BatchInterval     time.Duration
	BatchMaxItems     int
	QueueSize         int
}

// DefaultNATSConfig returns the default JetStream persistence config.
func DefaultNATSConfig() NATSConfig {
	cfg := config.Load().NATS
	return NATSConfig{
		URL:               cfg.URL,
		Stream:            cfg.Stream,
		EventSubject:      cfg.EventSubject,
		OrderSubject:      cfg.OrderSubject,
		TradeSubject:      cfg.TradeSubject,
		AsyncMaxPending:   cfg.AsyncMaxPending,
		PublishAckTimeout: cfg.PublishAckTimeout,
		BatchInterval:     cfg.BatchInterval,
		BatchMaxItems:     cfg.BatchMaxItems,
		QueueSize:         cfg.QueueSize,
	}
}

// JetStreamBatch contains mixed order and trade events.
type JetStreamBatch struct {
	Orders   []OrderEvent
	Trades   []TradeEvent
	Fundings []FundingEvent
}

type jetItem struct {
	order   *OrderEvent
	trade   *TradeEvent
	funding *FundingEvent
}

// JetStreamStore persists events to a JetStream stream.
type JetStreamStore struct {
	nc  *nats.Conn
	js  nats.JetStreamContext
	cfg NATSConfig
	seq uint64

	items   chan jetItem
	flushCh chan chan error
	done    chan struct{}
	wg      sync.WaitGroup
}

// OpenJetStreamStore connects to NATS and ensures the stream exists.
func OpenJetStreamStore(cfg NATSConfig) (*JetStreamStore, error) {
	if cfg.URL == "" {
		return nil, errors.New("nats url is required")
	}
	if cfg.Stream == "" {
		return nil, errors.New("stream name is required")
	}
	if cfg.EventSubject == "" {
		if cfg.OrderSubject != "" {
			cfg.EventSubject = cfg.OrderSubject
		} else {
			return nil, errors.New("event subject is required")
		}
	}

	nc, err := nats.Connect(cfg.URL)
	if err != nil {
		return nil, err
	}

	pending := cfg.AsyncMaxPending
	if pending <= 0 {
		pending = DefaultNATSConfig().AsyncMaxPending
	}

	js, err := nc.JetStream(nats.PublishAsyncMaxPending(pending))
	if err != nil {
		nc.Close()
		return nil, err
	}

	subjects := []string{cfg.EventSubject}
	if cfg.OrderSubject != "" && cfg.OrderSubject != cfg.EventSubject {
		subjects = append(subjects, cfg.OrderSubject)
	}
	if cfg.TradeSubject != "" && cfg.TradeSubject != cfg.EventSubject {
		subjects = append(subjects, cfg.TradeSubject)
	}

	_, err = js.StreamInfo(cfg.Stream)
	if err != nil {
		_, err = js.AddStream(&nats.StreamConfig{
			Name:      cfg.Stream,
			Subjects:  subjects,
			Retention: nats.LimitsPolicy,
			Storage:   nats.FileStorage,
		})
		if err != nil {
			nc.Close()
			return nil, err
		}
	}

	queueSize := cfg.QueueSize
	if queueSize <= 0 {
		queueSize = DefaultNATSConfig().QueueSize
	}

	store := &JetStreamStore{
		nc:      nc,
		js:      js,
		cfg:     cfg,
		items:   make(chan jetItem, queueSize),
		flushCh: make(chan chan error, 1),
		done:    make(chan struct{}),
	}
	store.wg.Add(1)
	go store.runBatcher()
	return store, nil
}

// AppendOrderCreated publishes a newly created order.
func (s *JetStreamStore) AppendOrderCreated(order *types.Order) error {
	return s.appendOrderEvent(ActionOrderCreated, order, types.Quantity{})
}

// AppendOrderAmended publishes an amendment event.
func (s *JetStreamStore) AppendOrderAmended(order *types.Order, newQty types.Quantity) error {
	return s.appendOrderEvent(ActionOrderAmended, order, newQty)
}

// AppendOrderCanceled publishes a cancellation event.
func (s *JetStreamStore) AppendOrderCanceled(order *types.Order) error {
	return s.appendOrderEvent(ActionOrderCanceled, order, types.Quantity{})
}

// AppendOrderUpdated publishes a post-match order update.
func (s *JetStreamStore) AppendOrderUpdated(order *types.Order) error {
	return s.appendOrderEvent(ActionOrderUpdated, order, types.Quantity{})
}

// AppendTrade publishes a trade execution event.
func (s *JetStreamStore) AppendTrade(trade types.Trade) error {
	event := TradeEvent{
		Seq:       s.nextSeq(),
		Timestamp: utils.NowNano(),
		Trade:     trade,
	}
	return s.enqueue(jetItem{trade: &event})
}

// AppendFunding publishes a funding lifecycle event.
func (s *JetStreamStore) AppendFunding(funding types.FundingRequest) error {
	event := FundingEvent{
		Seq:       s.nextSeq(),
		Timestamp: utils.NowNano(),
		Funding:   funding,
	}
	return s.enqueue(jetItem{funding: &event})
}

// Close waits for async publishes and drains the NATS connection.
func (s *JetStreamStore) Close() error {
	if s.js != nil {
		if err := s.Flush(); err != nil {
			_ = s.nc.Drain()
			return err
		}
	}
	close(s.done)
	s.wg.Wait()
	return s.nc.Drain()
}

// Flush drains the batch queue and waits for async publishes.
func (s *JetStreamStore) Flush() error {
	resp := make(chan error, 1)
	select {
	case s.flushCh <- resp:
		if err := <-resp; err != nil {
			return err
		}
	case <-s.done:
		return ErrPersistClosed
	}

	if s.cfg.PublishAckTimeout > 0 {
		select {
		case <-s.js.PublishAsyncComplete():
			return nil
		case <-time.After(s.cfg.PublishAckTimeout):
			return errors.New("nats publish flush timeout")
		}
	}
	return nil
}

// LastSeq reports the latest event sequence number.
func (s *JetStreamStore) LastSeq() uint64 {
	return atomic.LoadUint64(&s.seq)
}

// StreamLastSeq returns the last stream sequence.
func (s *JetStreamStore) StreamLastSeq() (uint64, error) {
	info, err := s.js.StreamInfo(s.cfg.Stream)
	if err != nil {
		return 0, err
	}
	return info.State.LastSeq, nil
}

// Replay replays JetStream events from the provided stream sequence.
func (s *JetStreamStore) Replay(fromSeq uint64, handler Handler) error {
	if handler == nil {
		return errors.New("replay handler is required")
	}
	if fromSeq == 0 {
		fromSeq = 1
	}

	info, err := s.js.StreamInfo(s.cfg.Stream)
	if err != nil {
		return err
	}
	last := info.State.LastSeq
	for seq := fromSeq; seq <= last; seq++ {
		msg, err := s.js.GetMsg(s.cfg.Stream, seq)
		if err != nil {
			return err
		}
		orders, trades, fundings, err := s.decodeJetStreamEvent(msg.Subject, msg.Data)
		if err != nil {
			return err
		}
		for _, order := range orders {
			if err := handler(Record{Kind: EventOrder, Order: &order}); err != nil {
				return err
			}
		}
		for _, trade := range trades {
			if err := handler(Record{Kind: EventTrade, Trade: &trade}); err != nil {
				return err
			}
		}
		for _, funding := range fundings {
			if err := handler(Record{Kind: EventFunding, Funding: &funding}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *JetStreamStore) appendOrderEvent(action ActionType, order *types.Order, newQty types.Quantity) error {
	event := OrderEvent{
		Seq:       s.nextSeq(),
		Action:    action,
		Timestamp: utils.NowNano(),
		Order:     *order,
		NewQty:    newQty,
	}
	return s.enqueue(jetItem{order: &event})
}

func (s *JetStreamStore) enqueue(item jetItem) error {
	if item.order == nil && item.trade == nil && item.funding == nil {
		return nil
	}
	select {
	case s.items <- item:
		return nil
	case <-s.done:
		return ErrPersistClosed
	}
}

func (s *JetStreamStore) runBatcher() {
	defer s.wg.Done()

	interval := s.cfg.BatchInterval
	if interval <= 0 {
		interval = DefaultNATSConfig().BatchInterval
	}
	maxItems := s.cfg.BatchMaxItems
	if maxItems <= 0 {
		maxItems = DefaultNATSConfig().BatchMaxItems
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	orders := make([]OrderEvent, 0, maxItems)
	trades := make([]TradeEvent, 0, maxItems)
	fundings := make([]FundingEvent, 0, maxItems)
	count := 0

	flush := func() error {
		if count == 0 {
			return nil
		}
		err := s.publishBatch(orders, trades, fundings)
		orders = orders[:0]
		trades = trades[:0]
		fundings = fundings[:0]
		count = 0
		return err
	}

	for {
		select {
		case item := <-s.items:
			if item.order != nil {
				orders = append(orders, *item.order)
				count++
			}
			if item.trade != nil {
				trades = append(trades, *item.trade)
				count++
			}
			if item.funding != nil {
				fundings = append(fundings, *item.funding)
				count++
			}
			if count >= maxItems {
				_ = flush()
			}

		case req := <-s.flushCh:
			for {
				select {
				case item := <-s.items:
					if item.order != nil {
						orders = append(orders, *item.order)
						count++
					}
					if item.trade != nil {
						trades = append(trades, *item.trade)
						count++
					}
					if item.funding != nil {
						fundings = append(fundings, *item.funding)
						count++
					}
					if count >= maxItems {
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

func (s *JetStreamStore) publishBatch(orders []OrderEvent, trades []TradeEvent, fundings []FundingEvent) error {
	if len(orders) == 0 && len(trades) == 0 && len(fundings) == 0 {
		return nil
	}

	batch := JetStreamBatch{Orders: orders, Trades: trades, Fundings: fundings}
	payload, err := encodeGob(batch)
	if err != nil {
		return err
	}
	ack, err := s.js.PublishAsync(s.cfg.EventSubject, payload)
	if err != nil {
		return err
	}
	if s.cfg.PublishAckTimeout > 0 {
		select {
		case <-ack.Ok():
			return nil
		case err := <-ack.Err():
			return fmt.Errorf("nats publish failed: %w", err)
		case <-time.After(s.cfg.PublishAckTimeout):
			return errors.New("nats publish ack timeout")
		}
	}
	return nil
}

func (s *JetStreamStore) nextSeq() uint64 {
	return atomic.AddUint64(&s.seq, 1)
}

func encodeGob(payload any) ([]byte, error) {
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	if err := encoder.Encode(payload); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (s *JetStreamStore) decodeJetStreamEvent(subject string, payload []byte) ([]OrderEvent, []TradeEvent, []FundingEvent, error) {
	if subject == "" {
		return nil, nil, nil, errors.New("nats subject is required")
	}

	decoder := gob.NewDecoder(bytes.NewReader(payload))
	if subject == s.cfg.EventSubject {
		var batch JetStreamBatch
		if err := decoder.Decode(&batch); err != nil {
			return nil, nil, nil, err
		}
		return batch.Orders, batch.Trades, batch.Fundings, nil
	}
	if subject == s.cfg.OrderSubject {
		var event OrderEvent
		if err := decoder.Decode(&event); err != nil {
			return nil, nil, nil, err
		}
		return []OrderEvent{event}, nil, nil, nil
	}
	if subject == s.cfg.TradeSubject {
		var event TradeEvent
		if err := decoder.Decode(&event); err != nil {
			return nil, nil, nil, err
		}
		return nil, []TradeEvent{event}, nil, nil
	}

	return nil, nil, nil, fmt.Errorf("unhandled subject: %s", subject)
}
