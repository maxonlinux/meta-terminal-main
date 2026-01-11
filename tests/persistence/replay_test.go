package persistence_test

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/persistence/snapshot"
	"github.com/anomalyco/meta-terminal-go/internal/persistence/wal"
)

func TestSnapshotAndWALReplaySequence(t *testing.T) {
	dir := t.TempDir()
	snapshotPath := filepath.Join(dir, "state.snap")
	walPath := filepath.Join(dir, "state.wal")

	snapWriter, err := snapshot.OpenFileWriter(snapshotPath, 32*1024)
	if err != nil {
		t.Fatalf("snapshot writer: %v", err)
	}
	snapshotPayload := []byte("snapshot-state-v1")
	if _, err := snapWriter.Write(snapshotPayload); err != nil {
		t.Fatalf("snapshot write: %v", err)
	}
	if err := snapWriter.Sync(); err != nil {
		t.Fatalf("snapshot sync: %v", err)
	}
	if err := snapWriter.Close(); err != nil {
		t.Fatalf("snapshot close: %v", err)
	}

	walWriter, err := wal.OpenFileWriter(walPath, 32*1024)
	if err != nil {
		t.Fatalf("wal writer: %v", err)
	}
	ops := [][]byte{[]byte("op1"), []byte("op2")}
	for _, op := range ops {
		if err := walWriter.Append(op); err != nil {
			t.Fatalf("wal append: %v", err)
		}
	}
	if err := walWriter.Sync(); err != nil {
		t.Fatalf("wal sync: %v", err)
	}
	if err := walWriter.Close(); err != nil {
		t.Fatalf("wal close: %v", err)
	}

	snapReader, err := snapshot.OpenFileReader(snapshotPath, 32*1024)
	if err != nil {
		t.Fatalf("snapshot reader: %v", err)
	}
	gotSnapshot, err := io.ReadAll(snapReader)
	if err != nil {
		t.Fatalf("snapshot read: %v", err)
	}
	if err := snapReader.Close(); err != nil {
		t.Fatalf("snapshot close: %v", err)
	}
	if string(gotSnapshot) != string(snapshotPayload) {
		t.Fatalf("snapshot mismatch: %s != %s", gotSnapshot, snapshotPayload)
	}

	walReader, err := wal.OpenFileReader(walPath, 32*1024)
	if err != nil {
		t.Fatalf("wal reader: %v", err)
	}
	var readOps [][]byte
	for {
		record, err := walReader.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("wal read: %v", err)
		}
		readOps = append(readOps, record)
	}
	if err := walReader.Close(); err != nil {
		t.Fatalf("wal close: %v", err)
	}
	if len(readOps) != len(ops) {
		t.Fatalf("wal op count mismatch: %d != %d", len(readOps), len(ops))
	}
	for i := range ops {
		if string(readOps[i]) != string(ops[i]) {
			t.Fatalf("wal op %d mismatch: %s != %s", i, readOps[i], ops[i])
		}
	}

	_ = os.Remove(snapshotPath)
	_ = os.Remove(walPath)
}
