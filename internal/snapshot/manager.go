package snapshot

import (
	"context"
	"sync"
	"time"

	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/internal/wal"
)

const (
	DefaultSnapshotInterval = 3 * time.Second
	DefaultEventThreshold   = 100
)

type Manager struct {
	snap       *Snapshot
	wal        *wal.WAL
	state      *state.State
	interval   time.Duration
	eventLimit int64
	stopCh     chan struct{}
	wg         sync.WaitGroup
	mu         sync.Mutex
	lastOffset int64
}

func NewManager(snap *Snapshot, wal *wal.WAL, state *state.State, interval time.Duration, eventLimit int64) *Manager {
	if interval == 0 {
		interval = DefaultSnapshotInterval
	}
	if eventLimit == 0 {
		eventLimit = DefaultEventThreshold
	}
	return &Manager{
		snap:       snap,
		wal:        wal,
		state:      state,
		interval:   interval,
		eventLimit: eventLimit,
		stopCh:     make(chan struct{}),
	}
}

func (m *Manager) Start(ctx context.Context) {
	m.wg.Add(1)
	go m.run(ctx)
}

func (m *Manager) Stop() {
	close(m.stopCh)
	m.wg.Wait()
}

func (m *Manager) run(ctx context.Context) {
	defer m.wg.Done()

	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.trySnapshot()
		}
	}
}

func (m *Manager) trySnapshot() {
	m.mu.Lock()
	eventCount := m.wal.EventCount()
	currentOffset := m.wal.Offset()
	m.mu.Unlock()

	if eventCount >= m.eventLimit || currentOffset > m.lastOffset {
		if err := m.snap.Create(m.state, currentOffset); err != nil {
			return
		}

		m.mu.Lock()
		m.lastOffset = currentOffset
		m.wal.ResetEventCount()
		m.mu.Unlock()

		m.truncateWAL(currentOffset)
	}
}

func (m *Manager) truncateWAL(offset int64) {
	if offset <= 0 {
		return
	}

	retainOffset := offset - 1024*100
	if retainOffset < 0 {
		retainOffset = 0
	}

	_ = m.wal.Truncate(retainOffset)
}
