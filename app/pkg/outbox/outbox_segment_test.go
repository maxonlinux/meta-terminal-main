package outbox

import (
	"path/filepath"
	"testing"
)

func TestSegmentRotation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "outbox.aol")
	log, err := openAppendLog(path, 256)
	if err != nil {
		t.Fatalf("open log: %v", err)
	}
	t.Cleanup(func() {
		_ = log.Close()
	})

	value := make([]byte, 200)
	for i := 0; i < 20; i++ {
		if _, err := log.Append(logRecordData, 1, value); err != nil {
			t.Fatalf("append: %v", err)
		}
	}
	if err := log.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	segments, err := listSegments(path)
	if err != nil {
		t.Fatalf("list segments: %v", err)
	}
	if len(segments) < 2 {
		t.Fatalf("expected segment rotation, got %d segments", len(segments))
	}
}
