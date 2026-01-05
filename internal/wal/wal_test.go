package wal

import (
	"testing"
)

func TestWALAppendAndIterate(t *testing.T) {
	tmpDir := t.TempDir()

	wal, err := New(tmpDir, 64)
	if err != nil {
		t.Fatalf("failed to create WAL: %v", err)
	}
	defer wal.Close()

	op1 := &Operation{
		Type:      OP_PLACE_ORDER,
		Timestamp: 1000,
		UserID:    1,
		Symbol:    1,
		OrderID:   100,
		Data:      []byte("order1"),
	}

	if err := wal.Append(op1); err != nil {
		t.Fatalf("failed to append: %v", err)
	}

	op2 := &Operation{
		Type:      OP_FILL,
		Timestamp: 2000,
		UserID:    2,
		Symbol:    1,
		OrderID:   200,
		Data:      []byte("fill1"),
	}

	if err := wal.Append(op2); err != nil {
		t.Fatalf("failed to append: %v", err)
	}

	wal.Flush()

	count := 0
	var lastTimestamp int64
	err = wal.IterateFrom(0, func(op *Operation) error {
		count++
		lastTimestamp = op.Timestamp
		return nil
	})

	if err != nil {
		t.Fatalf("failed to iterate: %v", err)
	}

	if count != 2 {
		t.Errorf("expected 2 operations, got %d", count)
	}

	if lastTimestamp != 2000 {
		t.Errorf("expected last timestamp 2000, got %d", lastTimestamp)
	}
}

func TestWALOffset(t *testing.T) {
	tmpDir := t.TempDir()

	wal, err := New(tmpDir, 64)
	if err != nil {
		t.Fatalf("failed to create WAL: %v", err)
	}
	defer wal.Close()

	offsetBefore := wal.Offset()

	op := &Operation{
		Type:      OP_CHECKPOINT,
		Timestamp: 1000,
		Data:      []byte("test"),
	}

	wal.Append(op)
	offsetAfter := wal.Offset()

	if offsetAfter <= offsetBefore {
		t.Error("offset should increase after append")
	}
}

func TestWALTruncate(t *testing.T) {
	tmpDir := t.TempDir()

	wal, err := New(tmpDir, 64)
	if err != nil {
		t.Fatalf("failed to create WAL: %v", err)
	}
	defer wal.Close()

	for i := 0; i < 10; i++ {
		op := &Operation{
			Type:      OP_CHECKPOINT,
			Timestamp: int64(i * 1000),
			Data:      []byte("test"),
		}
		wal.Append(op)
	}
	wal.Flush()

	offset := wal.Offset()
	wal.Truncate(offset / 2)

	count := 0
	wal.IterateFrom(0, func(op *Operation) error {
		count++
		return nil
	})

	if count == 10 {
		t.Error("should have fewer operations after truncate")
	}
}
