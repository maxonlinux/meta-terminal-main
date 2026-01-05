package outbox

import (
	"testing"
	"time"
)

func TestOutboxEnqueue(t *testing.T) {
	tmpDir := t.TempDir()

	outbox, err := New(tmpDir, 10, time.Hour)
	if err != nil {
		t.Fatalf("failed to create outbox: %v", err)
	}
	defer outbox.Close()

	outbox.Enqueue(Event{
		Type:      "trade",
		Timestamp: time.Now().Unix(),
		UserID:    1,
		Symbol:    1,
	})

	if outbox.BufferLen() != 1 {
		t.Errorf("expected buffer len 1, got %d", outbox.BufferLen())
	}
}

func TestOutboxAutoFlush(t *testing.T) {
	tmpDir := t.TempDir()

	outbox, err := New(tmpDir, 3, time.Hour)
	if err != nil {
		t.Fatalf("failed to create outbox: %v", err)
	}
	defer outbox.Close()

	outbox.Enqueue(Event{Type: "trade"})
	outbox.Enqueue(Event{Type: "trade"})
	outbox.Enqueue(Event{Type: "trade"})

	if outbox.BufferLen() != 0 {
		t.Errorf("expected buffer to flush, got %d", outbox.BufferLen())
	}
}

func TestOutboxFlush(t *testing.T) {
	tmpDir := t.TempDir()

	outbox, err := New(tmpDir, 100, time.Hour)
	if err != nil {
		t.Fatalf("failed to create outbox: %v", err)
	}
	defer outbox.Close()

	outbox.Enqueue(Event{Type: "test1"})
	outbox.Enqueue(Event{Type: "test2"})

	if outbox.BufferLen() != 2 {
		t.Errorf("expected 2 events in buffer, got %d", outbox.BufferLen())
	}

	outbox.Flush()

	if outbox.BufferLen() != 0 {
		t.Errorf("expected buffer to flush, got %d", outbox.BufferLen())
	}
}
