package outbox

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/maxonlinux/meta-terminal-go/pkg/events"
)

func TestCompactionCutoff(t *testing.T) {
	tests := []struct {
		name    string
		last    uint64
		pending map[uint64]uint64
		want    uint64
	}{
		{name: "no committed", last: 0, pending: nil, want: 0},
		{name: "no pending", last: 100, pending: nil, want: 100},
		{name: "pending after committed", last: 100, pending: map[uint64]uint64{1: 150}, want: 100},
		{name: "pending before committed", last: 100, pending: map[uint64]uint64{1: 90}, want: 89},
		{name: "earliest pending wins", last: 100, pending: map[uint64]uint64{1: 95, 2: 50, 3: 70}, want: 49},
		{name: "pending at first seq blocks compaction", last: 100, pending: map[uint64]uint64{1: 1}, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compactionCutoff(tt.last, tt.pending)
			if got != tt.want {
				t.Fatalf("compactionCutoff()=%d want=%d", got, tt.want)
			}
		})
	}
}

func TestReplayCompactsCommittedEvenWithPendingTx(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "outbox.aol")
	log, err := openAppendLog(logPath, 256, defaultLogFlushEvery, false, nil)
	if err != nil {
		t.Fatalf("open append log: %v", err)
	}

	committedTx := uint64(1)
	if _, err := log.Append(logRecordBegin, committedTx, nil); err != nil {
		t.Fatalf("append begin committed: %v", err)
	}
	if _, err := log.Append(logRecordData, committedTx, append([]byte{byte(events.OrderPlaced)}, []byte("ok")...)); err != nil {
		t.Fatalf("append data committed: %v", err)
	}
	if _, err := log.Append(logRecordCommit, committedTx, nil); err != nil {
		t.Fatalf("append commit committed: %v", err)
	}

	pendingTx := uint64(2)
	if _, err := log.Append(logRecordBegin, pendingTx, nil); err != nil {
		t.Fatalf("append begin pending: %v", err)
	}
	if _, err := log.Append(logRecordData, pendingTx, append([]byte{byte(events.OrderCanceled)}, []byte("pending")...)); err != nil {
		t.Fatalf("append data pending: %v", err)
	}

	if err := log.Flush(); err != nil {
		t.Fatalf("flush log: %v", err)
	}
	if err := log.Close(); err != nil {
		t.Fatalf("close log: %v", err)
	}

	ob, err := OpenWithOptions(dir, Options{ApplyBatchSize: 16})
	if err != nil {
		t.Fatalf("open outbox: %v", err)
	}
	ob.Start()
	defer func() {
		_ = ob.Close()
	}()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	segmentCount := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "outbox.aol.") {
			segmentCount++
		}
	}
	if segmentCount > 1 {
		t.Fatalf("expected compacted outbox with <=1 segment, got %d", segmentCount)
	}
}
