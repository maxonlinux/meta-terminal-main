package outbox

import (
	"errors"
	"io"
	"path/filepath"
	"testing"
)

func TestWriterReaderRoundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "outbox.log")

	writer, err := OpenWriter(path, 0)
	if err != nil {
		t.Fatalf("OpenWriter: %v", err)
	}
	if err := writer.Append(1, []byte("a")); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := writer.Append(2, []byte("bb")); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := writer.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := OpenReader(path, 0)
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	kind, payload, err := reader.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if kind != 1 || string(payload) != "a" {
		t.Fatalf("unexpected record: kind=%d payload=%q", kind, string(payload))
	}
	buf := make([]byte, 0, 8)
	kind, payload, err = reader.NextInto(buf)
	if err != nil {
		t.Fatalf("NextInto: %v", err)
	}
	if kind != 2 || string(payload) != "bb" {
		t.Fatalf("unexpected record: kind=%d payload=%q", kind, string(payload))
	}
	_, _, err = reader.Next()
	if !errors.Is(err, io.EOF) {
		t.Fatalf("expected EOF, got %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestReaderOffset(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "outbox.log")

	writer, err := OpenWriter(path, 0)
	if err != nil {
		t.Fatalf("OpenWriter: %v", err)
	}
	if err := writer.Append(1, []byte("abc")); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := writer.Append(2, []byte("defg")); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := OpenReader(path, 0)
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	_, _, err = reader.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	firstOffset := reader.Offset()
	_, _, err = reader.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	secondOffset := reader.Offset()
	if secondOffset <= firstOffset {
		t.Fatalf("expected offset to grow: first=%d second=%d", firstOffset, secondOffset)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}
