package outbox

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Event struct {
	Type      string
	Timestamp int64
	UserID    int64
	Symbol    int32
	Data      map[string]interface{}
}

type Outbox struct {
	path        string
	batchSize   int
	interval    time.Duration
	mu          sync.Mutex
	buffer      []Event
	flushTicker *time.Ticker
	done        chan struct{}
}

func New(path string, batchSize int, flushInterval time.Duration) (*Outbox, error) {
	if err := os.MkdirAll(path, 0755); err != nil {
		return nil, err
	}

	return &Outbox{
		path:      path,
		batchSize: batchSize,
		interval:  flushInterval,
		buffer:    make([]Event, 0, batchSize),
		done:      make(chan struct{}),
	}, nil
}

func (o *Outbox) Start() {
	o.flushTicker = time.NewTicker(o.interval)
	go func() {
		for {
			select {
			case <-o.flushTicker.C:
				o.Flush()
			case <-o.done:
				o.flushTicker.Stop()
				return
			}
		}
	}()
}

func (o *Outbox) Enqueue(event Event) {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.buffer = append(o.buffer, event)

	if len(o.buffer) >= o.batchSize {
		o.flushLocked()
	}
}

func (o *Outbox) Flush() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.flushLocked()
}

func (o *Outbox) flushLocked() {
	if len(o.buffer) == 0 {
		return
	}

	events := o.buffer
	o.buffer = make([]Event, 0, o.batchSize)

	go o.writeToDisk(events)
}

func (o *Outbox) writeToDisk(events []Event) {
	filename := filepath.Join(o.path, "outbox_1e7e3c6a.jsonl")

	data, _ := json.Marshal(events)
	os.WriteFile(filename+"_tmp", data, 0644)
	os.Rename(filename+"_tmp", filename)
}

func (o *Outbox) Close() {
	close(o.done)
	o.Flush()
}

func (o *Outbox) BufferLen() int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return len(o.buffer)
}
