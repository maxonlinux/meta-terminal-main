package persistence

import (
	"time"

	"github.com/anomalyco/meta-terminal-go/internal/engine"
	"github.com/anomalyco/meta-terminal-go/internal/snapshot"
	"github.com/anomalyco/meta-terminal-go/internal/types"
	"github.com/anomalyco/meta-terminal-go/internal/wal"
)

type Manager struct {
	engine       *engine.Engine
	wal          *wal.WAL
	snapshots    *snapshot.Store
	maxEvents    int
	maxSize      int64
	interval     time.Duration
	eventsSince  int
	lastSnapshot time.Time
	stopCh       chan struct{}
}

func New(engine *engine.Engine, walStore *wal.WAL, snapStore *snapshot.Store, maxEvents int, maxSize int64, interval time.Duration) *Manager {
	if maxEvents <= 0 {
		maxEvents = 1000
	}
	if maxSize <= 0 {
		maxSize = 1024 * 1024
	}
	if interval <= 0 {
		interval = 3 * time.Second
	}
	return &Manager{
		engine:       engine,
		wal:          walStore,
		snapshots:    snapStore,
		maxEvents:    maxEvents,
		maxSize:      maxSize,
		interval:     interval,
		lastSnapshot: time.Now(),
		stopCh:       make(chan struct{}),
	}
}

func (m *Manager) Start() {
	go func() {
		ticker := time.NewTicker(m.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if m.eventsSince > 0 {
					_ = m.Snapshot()
				}
			case <-m.stopCh:
				return
			}
		}
	}()
}

func (m *Manager) Stop() {
	close(m.stopCh)
	_ = m.Snapshot()
}

func (m *Manager) Append(eventType uint8, payload []byte) error {
	if _, err := m.wal.Append(eventType, payload); err != nil {
		return err
	}
	m.eventsSince++
	if m.eventsSince >= m.maxEvents || m.wal.Size() >= m.maxSize {
		return m.Snapshot()
	}
	return nil
}

func (m *Manager) Snapshot() error {
	snap := m.engine.Snapshot()
	if err := m.snapshots.Save(snap); err != nil {
		return err
	}
	m.eventsSince = 0
	m.lastSnapshot = time.Now()
	return m.wal.Truncate()
}

func (m *Manager) Replay() error {
	if m.snapshots.Exists() {
		snap, err := m.snapshots.Load()
		if err == nil {
			m.engine.ApplySnapshot(snap)
		}
	}

	return m.wal.Replay(func(evt wal.Event) error {
		return m.applyEvent(evt)
	})
}

func (m *Manager) applyEvent(evt wal.Event) error {
	switch evt.Type {
	case wal.EventPlaceOrder:
		orderID, input, err := wal.DecodePlaceOrder(evt.Payload)
		if err != nil {
			return err
		}
		result, _ := m.engine.PlaceOrderWithID(orderID, &input)
		m.engine.ReleaseResult(result)
	case wal.EventCancelOrder:
		orderID, userID, err := wal.DecodeCancelOrder(evt.Payload)
		if err != nil {
			return err
		}
		_ = m.engine.CancelOrder(orderID, userID)
	case wal.EventPriceTick:
		symbol, price, err := wal.DecodePriceTick(evt.Payload)
		if err != nil {
			return err
		}
		m.engine.OnPriceTick(symbol, price)
	case wal.EventSetLeverage:
		userID, symbol, leverage, err := wal.DecodeSetLeverage(evt.Payload)
		if err != nil {
			return err
		}
		_ = m.engine.SetLeverage(userID, symbol, leverage)
	case wal.EventSetBalance:
		userID, asset, amount, err := wal.DecodeSetBalance(evt.Payload)
		if err != nil {
			return err
		}
		_ = m.engine.SetBalance(userID, asset, amount)
	case wal.EventAddInstrument:
		symbol, base, quote, category, price, err := wal.DecodeAddInstrument(evt.Payload)
		if err != nil {
			return err
		}
		m.engine.AddInstrumentFull(symbol, base, quote, category, price)
	default:
		return nil
	}
	return nil
}

func PlaceOrderPayload(orderID types.OrderID, input *types.OrderInput) []byte {
	return wal.EncodePlaceOrder(orderID, input)
}

func CancelOrderPayload(orderID types.OrderID, userID types.UserID) []byte {
	return wal.EncodeCancelOrder(orderID, userID)
}

func PriceTickPayload(symbol string, price types.Price) []byte {
	return wal.EncodePriceTick(symbol, price)
}

func SetLeveragePayload(userID types.UserID, symbol string, leverage int8) []byte {
	return wal.EncodeSetLeverage(userID, symbol, leverage)
}

func SetBalancePayload(userID types.UserID, asset string, amount int64) []byte {
	return wal.EncodeSetBalance(userID, asset, amount)
}

func AddInstrumentPayload(symbol, base, quote string, category int8, price types.Price) []byte {
	return wal.EncodeAddInstrument(symbol, base, quote, category, price)
}
