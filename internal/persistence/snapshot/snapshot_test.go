package snapshot

import (
	"bytes"
	"path/filepath"
	"testing"
)

func TestFileWriterReader(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "snapshot.bin")

	writer, err := OpenFileWriter(path, 0)
	if err != nil {
		t.Fatalf("OpenFileWriter: %v", err)
	}
	payload := []byte("snapshot-data")
	if _, err := writer.Write(payload); err != nil {
		t.Fatalf("Write: %v", err)
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
	data, err := ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !bytes.Equal(data, payload) {
		t.Fatalf("payload mismatch: %q", string(data))
	}
}
