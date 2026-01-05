package wal

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
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
		f.Close()
		return nil, err
	}

	return &WAL{
		path:       path,
		file:       f,
		offset:     stat.Size(),
		bufferSize: bufferSize * 1024,
	}, nil
}

func (w *WAL) Append(op *Operation) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	data, err := json.Marshal(op)
	if err != nil {
		return err
	}

	header := make([]byte, 17)
	header[0] = byte(op.Type)
	binary.BigEndian.PutUint64(header[1:9], uint64(op.Timestamp))
	binary.BigEndian.PutUint64(header[9:17], uint64(len(data)))

	writer := bufio.NewWriterSize(w.file, w.bufferSize)
	if _, err := writer.Write(header); err != nil {
		return err
	}
	if _, err := writer.Write(data); err != nil {
		return err
	}
	writer.Flush()

	w.offset += int64(len(header) + len(data))
	return nil
}

func (w *WAL) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.file.Sync()
}

func (w *WAL) IterateFrom(offset int64, fn func(op *Operation) error) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	file, err := os.Open(w.path + "/wal.log")
	if err != nil {
		return err
	}
	defer file.Close()

	reader := bufio.NewReaderSize(file, w.bufferSize)
	if offset > 0 {
		reader.Discard(int(offset))
	}

	for {
		header := make([]byte, 17)
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
		}

		dataLen := int64(binary.BigEndian.Uint64(header[9:17]))
		data := make([]byte, dataLen)
		n, err = reader.Read(data)
		if err != nil {
			return err
		}

		if err := json.Unmarshal(data, op); err != nil {
			return err
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
