package wal

import (
	"encoding/binary"
	"errors"
	"io"
	"path/filepath"
	"testing"

	"github.com/maxonlinux/meta-terminal-go/internal/persistence/snapshot"
)

func TestFileWriterReader(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wal.log")

	writer, err := OpenFileWriter(path, 0)
	if err != nil {
		t.Fatalf("OpenFileWriter: %v", err)
	}
	if err := writer.Append([]byte("one")); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := writer.Append([]byte("two")); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := writer.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := OpenFileReader(path, 0)
	if err != nil {
		t.Fatalf("OpenFileReader: %v", err)
	}
	record, err := reader.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if string(record) != "one" {
		t.Fatalf("expected one, got %q", string(record))
	}
	buf := make([]byte, 0, 8)
	record, err = reader.NextInto(buf)
	if err != nil {
		t.Fatalf("NextInto: %v", err)
	}
	if string(record) != "two" {
		t.Fatalf("expected two, got %q", string(record))
	}
	_, err = reader.Next()
	if !errors.Is(err, io.EOF) {
		t.Fatalf("expected EOF, got %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestReplaySnapshotWithWAL(t *testing.T) {
	dir := t.TempDir()
	snapPath := filepath.Join(dir, "snapshot.bin")
	walPath := filepath.Join(dir, "wal.log")

	const snapValue int64 = 10
	snapWriter, err := snapshot.OpenFileWriter(snapPath, 0)
	if err != nil {
		t.Fatalf("OpenFileWriter snapshot: %v", err)
	}
	var snapBuf [8]byte
	binary.LittleEndian.PutUint64(snapBuf[:], uint64(snapValue))
	if _, err := snapWriter.Write(snapBuf[:]); err != nil {
		t.Fatalf("snapshot write: %v", err)
	}
	if err := snapWriter.Sync(); err != nil {
		t.Fatalf("snapshot sync: %v", err)
	}
	if err := snapWriter.Close(); err != nil {
		t.Fatalf("snapshot close: %v", err)
	}

	walWriter, err := OpenFileWriter(walPath, 0)
	if err != nil {
		t.Fatalf("OpenFileWriter wal: %v", err)
	}
	for _, delta := range []int64{1, 5, -2} {
		var buf [8]byte
		binary.LittleEndian.PutUint64(buf[:], uint64(delta))
		if err := walWriter.Append(buf[:]); err != nil {
			t.Fatalf("wal append: %v", err)
		}
	}
	if err := walWriter.Sync(); err != nil {
		t.Fatalf("wal sync: %v", err)
	}
	if err := walWriter.Close(); err != nil {
		t.Fatalf("wal close: %v", err)
	}

	snapReader, err := snapshot.OpenFileReader(snapPath, 0)
	if err != nil {
		t.Fatalf("OpenFileReader snapshot: %v", err)
	}
	snapData, err := snapshot.ReadAll(snapReader)
	if err != nil {
		t.Fatalf("snapshot read: %v", err)
	}
	if err := snapReader.Close(); err != nil {
		t.Fatalf("snapshot close: %v", err)
	}
	if len(snapData) != 8 {
		t.Fatalf("unexpected snapshot size: %d", len(snapData))
	}
	state := int64(binary.LittleEndian.Uint64(snapData))

	walReader, err := OpenFileReader(walPath, 0)
	if err != nil {
		t.Fatalf("OpenFileReader wal: %v", err)
	}
	for {
		record, err := walReader.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			t.Fatalf("wal next: %v", err)
		}
		if len(record) != 8 {
			t.Fatalf("invalid wal record size: %d", len(record))
		}
		delta := int64(binary.LittleEndian.Uint64(record))
		state += delta
	}
	if err := walReader.Close(); err != nil {
		t.Fatalf("wal close: %v", err)
	}

	if state != 14 {
		t.Fatalf("expected state=14, got %d", state)
	}
}
