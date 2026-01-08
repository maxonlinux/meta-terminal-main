package wal

import (
	"bufio"
	"encoding/binary"
	"os"
	"sync"
)

type OperationType int8

const (
	OP_PLACE_ORDER OperationType = iota
	OP_CANCEL_ORDER
	OP_AMEND_ORDER
	OP_FILL
	OP_POSITION_UPDATE
	OP_BALANCE_UPDATE
)

type Operation struct {
	Type      OperationType
	Timestamp int64
	UserID    int64
	Symbol    string
	OrderID   int64
}

type FillOp struct {
	TakerOrderID int64
	MakerOrderID int64
	TradePrice   int64
	TradeSize    int64
	Symbol       string
	TakerUserID  int64
	MakerUserID  int64
}

type PositionUpdateOp struct {
	UserID     int64
	Symbol     string
	Size       int64
	Side       int8
	EntryPrice int64
	Leverage   int8
}

type WAL struct {
	file     *os.File
	writer   *bufio.Writer
	mu       sync.Mutex
	position int64
}

func New(path string, bufferSize int) (*WAL, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}
	return &WAL{
		file:   f,
		writer: bufio.NewWriterSize(f, bufferSize),
	}, nil
}

func (w *WAL) Append(op *Operation) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := binary.Write(w.writer, binary.LittleEndian, op.Type); err != nil {
		return err
	}
	if err := binary.Write(w.writer, binary.LittleEndian, op.Timestamp); err != nil {
		return err
	}
	if err := binary.Write(w.writer, binary.LittleEndian, op.UserID); err != nil {
		return err
	}
	if err := writeString(w.writer, op.Symbol); err != nil {
		return err
	}
	if err := binary.Write(w.writer, binary.LittleEndian, op.OrderID); err != nil {
		return err
	}
	w.position++
	return w.writer.Flush()
}

func (w *WAL) AppendFill(op *FillOp) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := binary.Write(w.writer, binary.LittleEndian, OP_FILL); err != nil {
		return err
	}
	if err := binary.Write(w.writer, binary.LittleEndian, op.TakerOrderID); err != nil {
		return err
	}
	if err := binary.Write(w.writer, binary.LittleEndian, op.MakerOrderID); err != nil {
		return err
	}
	if err := binary.Write(w.writer, binary.LittleEndian, op.TradePrice); err != nil {
		return err
	}
	if err := binary.Write(w.writer, binary.LittleEndian, op.TradeSize); err != nil {
		return err
	}
	if err := writeString(w.writer, op.Symbol); err != nil {
		return err
	}
	if err := binary.Write(w.writer, binary.LittleEndian, op.TakerUserID); err != nil {
		return err
	}
	if err := binary.Write(w.writer, binary.LittleEndian, op.MakerUserID); err != nil {
		return err
	}
	return w.writer.Flush()
}

func (w *WAL) Close() error {
	return w.file.Close()
}

func writeString(w *bufio.Writer, s string) error {
	if err := binary.Write(w, binary.LittleEndian, int32(len(s))); err != nil {
		return err
	}
	_, err := w.WriteString(s)
	return err
}
