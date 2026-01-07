package wal

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

type OperationType byte

const (
	OP_PLACE_ORDER OperationType = iota
	OP_CANCEL_ORDER
	OP_AMEND_ORDER
	OP_FILL
	OP_POSITION_UPDATE
	OP_BALANCE_UPDATE
	OP_CHECKPOINT
)

type FillOp struct {
	TakerOrderID int64
	MakerOrderID int64
	Price        int64
	Quantity     int64
	Symbol       int32
	TakerUserID  int64
	MakerUserID  int64
}

type PositionUpdateOp struct {
	UserID     int64
	Symbol     int32
	Size       int64
	Side       int8
	EntryPrice int64
	Leverage   int8
}

type BalanceUpdateOp struct {
	UserID    int64
	Asset     string
	Available int64
	Locked    int64
	Margin    int64
}

type Operation struct {
	Type      OperationType
	Timestamp int64
	UserID    int64
	Symbol    int32
	OrderID   int64
	Data      []byte
}

type WAL struct {
	path       string
	file       *os.File
	offset     int64
	mu         sync.Mutex
	bufferSize int
	writeBuf   []byte
	eventCount int64
}

func New(path string, bufferSize int) (*WAL, error) {
	if err := os.MkdirAll(path, 0755); err != nil {
		return nil, fmt.Errorf("failed to create WAL dir: %w", err)
	}

	f, err := os.OpenFile(filepath.Join(path, "wal.log"), os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open WAL file: %w", err)
	}

	stat, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}

	return &WAL{
		path:       path,
		file:       f,
		offset:     stat.Size(),
		bufferSize: bufferSize * 1024,
		writeBuf:   make([]byte, 0, 1024),
	}, nil
}

func (w *WAL) Append(op *Operation) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	var data []byte
	if op.Data != nil {
		data = op.Data
	}

	headerLen := 33
	totalLen := headerLen + len(data)
	if cap(w.writeBuf) < totalLen {
		w.writeBuf = make([]byte, totalLen)
	} else {
		w.writeBuf = w.writeBuf[:totalLen]
	}

	w.writeBuf[0] = byte(op.Type)
	binary.BigEndian.PutUint64(w.writeBuf[1:9], uint64(op.Timestamp))
	binary.BigEndian.PutUint64(w.writeBuf[9:17], uint64(op.UserID))
	binary.BigEndian.PutUint32(w.writeBuf[17:21], uint32(op.Symbol))
	binary.BigEndian.PutUint64(w.writeBuf[21:29], uint64(op.OrderID))
	binary.BigEndian.PutUint32(w.writeBuf[29:33], uint32(len(data)))

	if len(data) > 0 {
		copy(w.writeBuf[33:], data)
	}

	_, err := w.file.Write(w.writeBuf)
	if err != nil {
		return err
	}

	w.offset += int64(totalLen)
	atomic.AddInt64(&w.eventCount, 1)
	return nil
}

func (w *WAL) AppendFill(op *FillOp) error {
	data := make([]byte, 56)
	binary.BigEndian.PutUint64(data[0:8], uint64(op.TakerOrderID))
	binary.BigEndian.PutUint64(data[8:16], uint64(op.MakerOrderID))
	binary.BigEndian.PutUint64(data[16:24], uint64(op.Price))
	binary.BigEndian.PutUint64(data[24:32], uint64(op.Quantity))
	binary.BigEndian.PutUint32(data[32:36], uint32(op.Symbol))
	binary.BigEndian.PutUint64(data[36:44], uint64(op.TakerUserID))
	binary.BigEndian.PutUint64(data[44:52], uint64(op.MakerUserID))
	return w.appendOp(OP_FILL, op.TakerUserID, op.Symbol, op.TakerOrderID, data)
}

func (w *WAL) AppendPositionUpdate(op *PositionUpdateOp) error {
	data := make([]byte, 32)
	binary.BigEndian.PutUint64(data[0:8], uint64(op.UserID))
	binary.BigEndian.PutUint32(data[8:12], uint32(op.Symbol))
	binary.BigEndian.PutUint64(data[12:20], uint64(op.Size))
	data[20] = byte(op.Side)
	binary.BigEndian.PutUint64(data[21:29], uint64(op.EntryPrice))
	data[29] = byte(op.Leverage)
	return w.appendOp(OP_POSITION_UPDATE, op.UserID, op.Symbol, 0, data)
}

func (w *WAL) AppendBalanceUpdate(op *BalanceUpdateOp) error {
	assetLen := len(op.Asset)
	data := make([]byte, 32+assetLen)
	binary.BigEndian.PutUint64(data[0:8], uint64(op.UserID))
	binary.BigEndian.PutUint64(data[8:16], uint64(op.Available))
	binary.BigEndian.PutUint64(data[16:24], uint64(op.Locked))
	binary.BigEndian.PutUint64(data[24:32], uint64(op.Margin))
	copy(data[32:], op.Asset)
	return w.appendOp(OP_BALANCE_UPDATE, op.UserID, 0, 0, data)
}

func (w *WAL) appendOp(opType OperationType, userID int64, symbol int32, orderID int64, data []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	headerLen := 33
	totalLen := headerLen + len(data)
	if cap(w.writeBuf) < totalLen {
		w.writeBuf = make([]byte, totalLen)
	} else {
		w.writeBuf = w.writeBuf[:totalLen]
	}

	w.writeBuf[0] = byte(opType)
	binary.BigEndian.PutUint64(w.writeBuf[1:9], uint64(time.Now().UnixNano()))
	binary.BigEndian.PutUint64(w.writeBuf[9:17], uint64(userID))
	binary.BigEndian.PutUint32(w.writeBuf[17:21], uint32(symbol))
	binary.BigEndian.PutUint64(w.writeBuf[21:29], uint64(orderID))
	binary.BigEndian.PutUint32(w.writeBuf[29:33], uint32(len(data)))

	if len(data) > 0 {
		copy(w.writeBuf[33:], data)
	}

	_, err := w.file.Write(w.writeBuf)
	if err != nil {
		return err
	}

	w.offset += int64(totalLen)
	atomic.AddInt64(&w.eventCount, 1)
	return nil
}

func (w *WAL) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.file.Sync()
}

func (w *WAL) ReadOperation(offset int64) (*Operation, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	file, err := os.Open(w.path + "/wal.log")
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	_, err = file.Seek(offset, 0)
	if err != nil {
		return nil, err
	}

	header := make([]byte, 33)
	n, err := file.Read(header)
	if err != nil {
		return nil, err
	}
	if n < 33 {
		return nil, fmt.Errorf("incomplete header")
	}

	op := &Operation{
		Type:      OperationType(header[0]),
		Timestamp: int64(binary.BigEndian.Uint64(header[1:9])),
		UserID:    int64(binary.BigEndian.Uint64(header[9:17])),
		Symbol:    int32(binary.BigEndian.Uint32(header[17:21])),
		OrderID:   int64(binary.BigEndian.Uint64(header[21:29])),
	}

	dataLen := int(binary.BigEndian.Uint32(header[29:33]))
	if dataLen > 0 {
		op.Data = make([]byte, dataLen)
		n, err = file.Read(op.Data)
		if err != nil {
			return nil, err
		}
		if n < dataLen {
			return nil, fmt.Errorf("incomplete data")
		}
	}

	return op, nil
}

func (w *WAL) IterateFrom(offset int64, fn func(op *Operation) error) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	file, err := os.Open(w.path + "/wal.log")
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	reader := bufio.NewReaderSize(file, w.bufferSize)
	if offset > 0 {
		_, _ = reader.Discard(int(offset))
	}

	for {
		header := make([]byte, 33)
		n, err := reader.Read(header)
		if n == 0 && err != nil {
			break
		}
		if err != nil {
			return err
		}

		op := &Operation{
			Type:      OperationType(header[0]),
			Timestamp: int64(binary.BigEndian.Uint64(header[1:9])),
			UserID:    int64(binary.BigEndian.Uint64(header[9:17])),
			Symbol:    int32(binary.BigEndian.Uint32(header[17:21])),
			OrderID:   int64(binary.BigEndian.Uint64(header[21:29])),
		}

		dataLen := int(binary.BigEndian.Uint32(header[29:33]))
		if dataLen > 0 {
			op.Data = make([]byte, dataLen)
			n, err = reader.Read(op.Data)
			if err != nil {
				return err
			}
			if n < dataLen {
				return fmt.Errorf("incomplete data")
			}
		}

		if err := fn(op); err != nil {
			return err
		}
	}

	return nil
}

func (w *WAL) Truncate(offset int64) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.file.Truncate(offset)
}

func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.file.Close()
}

func (w *WAL) Offset() int64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.offset
}

func (w *WAL) EventCount() int64 {
	return atomic.LoadInt64(&w.eventCount)
}

func (w *WAL) ResetEventCount() {
	atomic.StoreInt64(&w.eventCount, 0)
}
