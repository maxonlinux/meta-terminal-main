package outbox

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/maxonlinux/meta-terminal-go/pkg/events"
)

type recordingSink struct {
	mu      sync.Mutex
	batches [][]events.Event
}

func (s *recordingSink) Apply(batch []events.Event) error {
	copyBatch := make([]events.Event, len(batch))
	copy(copyBatch, batch)
	s.mu.Lock()
	s.batches = append(s.batches, copyBatch)
	s.mu.Unlock()
	return nil
}

func (s *recordingSink) flatten() []events.Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []events.Event
	for i := range s.batches {
		out = append(out, s.batches[i]...)
	}
	return out
}

func waitForEvents(t *testing.T, sink *recordingSink, want int) []events.Event {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		got := sink.flatten()
		if len(got) >= want {
			return got
		}
		time.Sleep(5 * time.Millisecond)
	}
	return sink.flatten()
}

func TestLiveAndReplayUseSameApplyFlow(t *testing.T) {
	dataA := []byte{1, 2, 3}
	dataB := []byte{4, 5}
	want := []events.Event{
		{Type: events.OrderCanceled, Data: dataA},
		{Type: events.RPNLRecorded, Data: dataB},
	}

	liveDir := t.TempDir()
	liveSink := &recordingSink{}
	live, err := OpenWithOptions(liveDir, Options{EventSink: liveSink, ApplyBatchSize: 16, ApplyBatchFlushEvery: time.Millisecond})
	if err != nil {
		t.Fatalf("open live outbox: %v", err)
	}
	live.Start()
	t.Cleanup(func() {
		_ = live.Close()
	})

	tx := live.Begin()
	if tx == nil {
		t.Fatalf("begin tx: nil")
	}
	if err := tx.Record(want[0]); err != nil {
		t.Fatalf("record first event: %v", err)
	}
	if err := tx.Record(want[1]); err != nil {
		t.Fatalf("record second event: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit tx: %v", err)
	}

	liveEvents := waitForEvents(t, liveSink, len(want))
	if len(liveEvents) != len(want) {
		t.Fatalf("live applied %d events, want %d", len(liveEvents), len(want))
	}
	for i := range want {
		if liveEvents[i].Type != want[i].Type || string(liveEvents[i].Data) != string(want[i].Data) {
			t.Fatalf("live event[%d] mismatch", i)
		}
	}

	replayDir := t.TempDir()
	logPath := filepath.Join(replayDir, "outbox.aol")
	log, err := openAppendLog(logPath, defaultSegmentSize, defaultLogFlushEvery, false, nil)
	if err != nil {
		t.Fatalf("open append log: %v", err)
	}
	txID := uint64(42)
	if _, err := log.Append(logRecordBegin, txID, nil); err != nil {
		t.Fatalf("append begin: %v", err)
	}
	if _, err := log.Append(logRecordData, txID, append([]byte{byte(want[0].Type)}, want[0].Data...)); err != nil {
		t.Fatalf("append data A: %v", err)
	}
	if _, err := log.Append(logRecordData, txID, append([]byte{byte(want[1].Type)}, want[1].Data...)); err != nil {
		t.Fatalf("append data B: %v", err)
	}
	if _, err := log.Append(logRecordCommit, txID, nil); err != nil {
		t.Fatalf("append commit: %v", err)
	}
	if err := log.Flush(); err != nil {
		t.Fatalf("flush log: %v", err)
	}
	if err := log.Close(); err != nil {
		t.Fatalf("close log: %v", err)
	}

	replaySink := &recordingSink{}
	replay, err := OpenWithOptions(replayDir, Options{EventSink: replaySink, ApplyBatchSize: 16})
	if err != nil {
		t.Fatalf("open replay outbox: %v", err)
	}
	replay.Start()
	defer func() {
		_ = replay.Close()
	}()

	replayEvents := replaySink.flatten()
	if len(replayEvents) != len(want) {
		t.Fatalf("replay applied %d events, want %d", len(replayEvents), len(want))
	}
	for i := range want {
		if replayEvents[i].Type != want[i].Type || string(replayEvents[i].Data) != string(want[i].Data) {
			t.Fatalf("replay event[%d] mismatch", i)
		}
	}
	for i := range want {
		if replayEvents[i].Type != liveEvents[i].Type || string(replayEvents[i].Data) != string(liveEvents[i].Data) {
			t.Fatalf("live/replay divergence at index %d", i)
		}
	}
}
