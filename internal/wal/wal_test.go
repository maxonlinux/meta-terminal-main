package wal

import (
	"path/filepath"
	"testing"
)

func TestAppendReplay(t *testing.T) {
	dir := t.TempDir()
	w, err := New(filepath.Join(dir, "wal"), 1024*1024, 64*1024)
	if err != nil {
		t.Fatalf("wal init failed: %v", err)
	}
	payload := []byte("payload")
	if _, err := w.Append(EventPriceTick, payload); err != nil {
		t.Fatalf("append failed: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}

	w, err = New(filepath.Join(dir, "wal"), 1024*1024, 64*1024)
	if err != nil {
		t.Fatalf("wal reopen failed: %v", err)
	}
	count := 0
	err = w.Replay(func(evt Event) error {
		count++
		if evt.Type != EventPriceTick {
			t.Fatalf("unexpected event type")
		}
		if string(evt.Payload) != string(payload) {
			t.Fatalf("unexpected payload")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("replay failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 event, got %d", count)
	}
}
