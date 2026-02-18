package outbox

import (
	"sync"
	"time"

	"github.com/maxonlinux/meta-terminal-go/pkg/events"
)

type BatchOptions struct {
	BatchSize  int
	FlushEvery time.Duration
}

type BatchSink struct {
	sink   EventSink
	mu     sync.Mutex
	batch  []events.Event
	ticker *time.Ticker
	stop   chan struct{}
}

func NewBatchSink(sink EventSink, opts BatchOptions) *BatchSink {
	if opts.BatchSize <= 0 {
		opts.BatchSize = 1000
	}
	bs := &BatchSink{
		sink:  sink,
		batch: make([]events.Event, 0, opts.BatchSize),
		stop:  make(chan struct{}),
	}
	if opts.FlushEvery > 0 {
		bs.ticker = time.NewTicker(opts.FlushEvery)
		go bs.flushLoop()
	}
	return bs
}

func (b *BatchSink) Apply(eventsBatch []events.Event) error {
	if b == nil || b.sink == nil || len(eventsBatch) == 0 {
		return nil
	}
	b.mu.Lock()
	b.batch = append(b.batch, eventsBatch...)
	if len(b.batch) < cap(b.batch) {
		b.mu.Unlock()
		return nil
	}
	batch := append([]events.Event(nil), b.batch...)
	b.batch = b.batch[:0]
	b.mu.Unlock()
	return b.sink.Apply(batch)
}

func (b *BatchSink) Flush() error {
	if b == nil || b.sink == nil {
		return nil
	}
	b.mu.Lock()
	if len(b.batch) == 0 {
		b.mu.Unlock()
		return nil
	}
	batch := append([]events.Event(nil), b.batch...)
	b.batch = b.batch[:0]
	b.mu.Unlock()
	return b.sink.Apply(batch)
}

func (b *BatchSink) Stop() error {
	if b == nil {
		return nil
	}
	if b.ticker != nil {
		b.ticker.Stop()
	}
	select {
	case <-b.stop:
	default:
		close(b.stop)
	}
	return b.Flush()
}

func (b *BatchSink) flushLoop() {
	for {
		select {
		case <-b.ticker.C:
			_ = b.Flush()
		case <-b.stop:
			return
		}
	}
}
