package wal

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sync"
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

// Binary operation format (fixed 32 bytes header + variable data)
// Header: Type(1) + Timestamp(8) + UserID(8) + Symbol(4) + OrderID(8) + DataLen(4) = 33 bytes
type BinaryOperation struct {
	Type      OperationType
	Timestamp uint64
	UserID    uint64
	Symbol    uint32
	OrderID   uint64
	DataLen   uint32
}

// Legacy Operation struct for compatibility during migration
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

	// Write binary directly - no JSON allocation
	var data []byte
	if op.Data != nil {
		data = op.Data
	}

	// Ensure buffer is large enough
	headerLen := 33
	totalLen := headerLen + len(data)
	if cap(w.writeBuf) < totalLen {
		w.writeBuf = make([]byte, totalLen)
	} else {
		w.writeBuf = w.writeBuf[:totalLen]
	}

	// Write header (binary, no allocation)
	w.writeBuf[0] = byte(op.Type)
	binary.BigEndian.PutUint64(w.writeBuf[1:9], uint64(op.Timestamp))
	binary.BigEndian.PutUint64(w.writeBuf[9:17], uint64(op.UserID))
	binary.BigEndian.PutUint32(w.writeBuf[17:21], uint32(op.Symbol))
	binary.BigEndian.PutUint64(w.writeBuf[21:29], uint64(op.OrderID))
	binary.BigEndian.PutUint32(w.writeBuf[29:33], uint32(len(data)))

	// Copy data
	if len(data) > 0 {
		copy(w.writeBuf[33:], data)
	}

	// Write to file
	_, err := w.file.Write(w.writeBuf)
	if err != nil {
		return err
	}

	w.offset += int64(totalLen)
	return nil
}

// AppendBinary - zero-allocation append using pre-serialized binary data
func (w *WAL) AppendBinary(data []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if len(data) < 33 {
		return fmt.Errorf("invalid binary operation data")
	}

	_, err := w.file.Write(data)
	if err != nil {
		return err
	}

	w.offset += int64(len(data))
	return nil
}

func (w *WAL) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.file.Sync()
}

// ReadOperation reads a binary operation from WAL
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

	// Read header
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
		// Read header
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
