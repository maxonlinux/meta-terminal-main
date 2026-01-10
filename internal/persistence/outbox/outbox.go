package outbox

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/messaging"
	"github.com/anomalyco/meta-terminal-go/internal/persistence/duckdb"
	"github.com/anomalyco/meta-terminal-go/internal/snowflake"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

type OutboxType uint8

const (
	OutboxTypeOrder OutboxType = iota
	OutboxTypeTrade
	OutboxTypeRPNL
)

type OutboxEntry struct {
	Type      OutboxType
	Timestamp uint64
	Data      []byte
}

type Config struct {
	Dir           string
	FlushInterval time.Duration
	FlushSize     int
	NATS          *messaging.NATS
}

type FileOutbox struct {
	cfg     Config
	mu      sync.RWMutex
	files   map[OutboxType]*os.File
	writers map[OutboxType]*bufio.Writer

	done chan struct{}
}

func New(cfg Config) (*FileOutbox, error) {
	if err := os.MkdirAll(cfg.Dir, 0755); err != nil {
		return nil, fmt.Errorf("create outbox dir: %w", err)
	}

	fo := &FileOutbox{
		cfg:     cfg,
		files:   make(map[OutboxType]*os.File),
		writers: make(map[OutboxType]*bufio.Writer),
		done:    make(chan struct{}),
	}

	// Open or create files for each type
	for t := OutboxTypeOrder; t <= OutboxTypeRPNL; t++ {
		path := filepath.Join(cfg.Dir, outboxFileName(t))
		f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("open outbox file %s: %w", path, err)
		}
		fo.files[t] = f
		fo.writers[t] = bufio.NewWriterSize(f, 64*1024) // 64KB buffer
	}

	// Start background flusher
	go fo.backgroundFlush()

	if fo.cfg.NATS != nil {
		go fo.startSubscriptions(context.Background())
	}

	return fo, nil
}

func (fo *FileOutbox) startSubscriptions(ctx context.Context) {
	fo.cfg.NATS.Subscribe(ctx, messaging.OrderEventTopic(""), "outbox-order", fo.handleOrderEvent)
	fo.cfg.NATS.Subscribe(ctx, messaging.SubjectClearingTrade, "outbox-trade", fo.handleTradeEvent)
	fo.cfg.NATS.Subscribe(ctx, messaging.SubjectPositionReduced, "outbox-rpnl", fo.handleRPNLEvent)
}

func (fo *FileOutbox) handleOrderEvent(data []byte) {
	var event types.OrderEvent
	dec := gob.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&event); err != nil {
		return
	}

	order := &types.Order{
		ID:           event.OrderID,
		UserID:       event.UserID,
		Symbol:       event.Symbol,
		Category:     event.Category,
		Side:         event.Side,
		Type:         event.Type,
		TIF:          event.TIF,
		Status:       event.Status,
		Price:        event.Price,
		Quantity:     event.Quantity,
		Filled:       event.Filled,
		TriggerPrice: event.TriggerPrice,
		ReduceOnly:   event.ReduceOnly,
		CreatedAt:    event.CreatedAt,
		UpdatedAt:    event.UpdatedAt,
	}
	fo.WriteOrder(context.Background(), order)
}

func (fo *FileOutbox) handleTradeEvent(data []byte) {
	var event types.TradeEvent
	dec := gob.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&event); err != nil {
		return
	}

	trade := &types.Trade{
		ID:           event.TradeID,
		Symbol:       event.Symbol,
		Category:     event.Category,
		TakerID:      event.TakerID,
		MakerID:      event.MakerID,
		TakerOrderID: event.TakerOrderID,
		MakerOrderID: event.MakerOrderID,
		Price:        event.Price,
		Quantity:     event.Quantity,
		ExecutedAt:   event.ExecutedAt,
	}
	fo.WriteTrade(context.Background(), trade)
}

func (fo *FileOutbox) handleRPNLEvent(data []byte) {
	var event types.PositionReducedEvent
	dec := gob.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&event); err != nil {
		return
	}

	rpnl := &types.RPNLEvent{
		ID:           uint64(snowflake.Next()),
		UserID:       event.UserID,
		Symbol:       event.Symbol,
		Category:     event.Category,
		RealizedPnl:  event.RPNL,
		PositionSize: event.PositionSize,
		PositionSide: event.PositionSide,
		EntryPrice:   0,
		ExitPrice:    event.ExitPrice,
		ExecutedAt:   event.ExecutedAt,
	}
	fo.WriteRPNL(context.Background(), rpnl)
}

func outboxFileName(t OutboxType) string {
	switch t {
	case OutboxTypeOrder:
		return "outbox_orders.bin"
	case OutboxTypeTrade:
		return "outbox_trades.bin"
	case OutboxTypeRPNL:
		return "outbox_rpnl.bin"
	default:
		return "outbox_unknown.bin"
	}
}

func (fo *FileOutbox) WriteOrder(ctx context.Context, order *types.Order) error {
	return fo.write(ctx, OutboxTypeOrder, order)
}

func (fo *FileOutbox) WriteTrade(ctx context.Context, trade *types.Trade) error {
	return fo.write(ctx, OutboxTypeTrade, trade)
}

func (fo *FileOutbox) WriteRPNL(ctx context.Context, rpnl *types.RPNLEvent) error {
	return fo.write(ctx, OutboxTypeRPNL, rpnl)
}

func (fo *FileOutbox) write(ctx context.Context, outboxType OutboxType, v any) error {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("gob encode: %w", err)
	}

	// Binary format: type(1) + timestamp(8) + data_len(4) + data
	header := make([]byte, 13)
	header[0] = byte(outboxType)
	binary.LittleEndian.PutUint64(header[1:9], types.NowNano())
	binary.LittleEndian.PutUint32(header[9:13], uint32(buf.Len()))

	fo.mu.RLock()
	writer, ok := fo.writers[outboxType]
	fo.mu.RUnlock()

	if !ok {
		return fmt.Errorf("outbox type %d not found", outboxType)
	}

	_, err := writer.Write(header)
	if err != nil {
		return fmt.Errorf("write header: %w", err)
	}

	_, err = writer.Write(buf.Bytes())
	if err != nil {
		return fmt.Errorf("write data: %w", err)
	}

	return nil
}

func (fo *FileOutbox) ReadAll(limit int) (map[OutboxType][]OutboxEntry, error) {
	result := make(map[OutboxType][]OutboxEntry)

	fo.mu.RLock()
	defer fo.mu.RUnlock()

	for t, f := range fo.files {
		// Flush first to get all data
		writer := fo.writers[t]
		writer.Flush()

		// Seek to beginning
		_, err := f.Seek(0, 0)
		if err != nil {
			continue
		}

		reader := bufio.NewReader(f)
		count := 0

		for count < limit {
			// Read type
			typeByte, err := reader.ReadByte()
			if err != nil {
				break
			}

			// Read timestamp
			timestampBuf := make([]byte, 8)
			_, err = reader.Read(timestampBuf)
			if err != nil {
				break
			}
			timestamp := binary.LittleEndian.Uint64(timestampBuf)

			// Read data length
			lenBuf := make([]byte, 4)
			_, err = reader.Read(lenBuf)
			if err != nil {
				break
			}
			dataLen := binary.LittleEndian.Uint32(lenBuf)

			// Read data
			data := make([]byte, dataLen)
			_, err = reader.Read(data)
			if err != nil {
				break
			}

			result[t] = append(result[t], OutboxEntry{
				Type:      OutboxType(typeByte),
				Timestamp: timestamp,
				Data:      data,
			})
			count++
		}
	}

	return result, nil
}

func (fo *FileOutbox) Flush() {
	fo.mu.RLock()
	defer fo.mu.RUnlock()

	for _, writer := range fo.writers {
		writer.Flush()
	}
}

func (fo *FileOutbox) Close() error {
	close(fo.done)

	fo.mu.Lock()
	defer fo.mu.Unlock()

	for t, f := range fo.files {
		if writer, ok := fo.writers[t]; ok {
			writer.Flush()
		}
		f.Close()
	}
	return nil
}

func (fo *FileOutbox) backgroundFlush() {
	ticker := time.NewTicker(fo.cfg.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-fo.done:
			return
		case <-ticker.C:
			fo.Flush()
		}
	}
}

type OutboxBatchWriter struct {
	outbox  *FileOutbox
	repo    *duckdb.Repository
	mu      sync.Mutex
	pending map[OutboxType]bool
}

func NewBatchWriter(outbox *FileOutbox, repo *duckdb.Repository) *OutboxBatchWriter {
	return &OutboxBatchWriter{
		outbox:  outbox,
		repo:    repo,
		pending: make(map[OutboxType]bool),
	}
}

func (bw *OutboxBatchWriter) Start(ctx context.Context) error {
	go bw.run(ctx)
	return nil
}

func (bw *OutboxBatchWriter) run(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(constants.BATCH_FLUSH_INTERVAL_MS) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			bw.flush()
		}
	}
}

func (bw *OutboxBatchWriter) flush() {
	bw.mu.Lock()
	defer bw.mu.Unlock()

	entries, err := bw.outbox.ReadAll(constants.BATCH_FLUSH_SIZE)
	if err != nil {
		return
	}

	if len(entries) == 0 {
		return
	}

	for outboxType, outEntries := range entries {
		if len(outEntries) == 0 {
			continue
		}

		var data bytes.Buffer
		for _, e := range outEntries {
			data.Write(e.Data)
		}

		switch outboxType {
		case OutboxTypeOrder:
			orders, err := duckdb.DecodeOutboxOrders(data.Bytes())
			if err != nil {
				continue
			}
			if err := bw.repo.BatchInsertOutboxOrders(orders); err != nil {
				continue
			}
		case OutboxTypeTrade:
			trades, err := duckdb.DecodeOutboxTrades(data.Bytes())
			if err != nil {
				continue
			}
			if err := bw.repo.BatchInsertOutboxTrades(trades); err != nil {
				continue
			}
		case OutboxTypeRPNL:
			rpnls, err := duckdb.DecodeOutboxRPNLs(data.Bytes())
			if err != nil {
				continue
			}
			if err := bw.repo.BatchInsertOutboxRPNL(rpnls); err != nil {
				continue
			}
		}
	}
}
