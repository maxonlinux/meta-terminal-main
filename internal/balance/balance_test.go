package balance

import (
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
)

func TestAddDeduct(t *testing.T) {
	b := New()
	b.Add(constants.BUCKET_AVAILABLE, 1000)
	if b.Get(constants.BUCKET_AVAILABLE) != 1000 {
		t.Fatalf("expected available 1000")
	}
	if ok := b.Deduct(constants.BUCKET_AVAILABLE, 500); !ok {
		t.Fatalf("expected deduct ok")
	}
	if b.Get(constants.BUCKET_AVAILABLE) != 500 {
		t.Fatalf("expected available 500")
	}
	if ok := b.Deduct(constants.BUCKET_AVAILABLE, 600); ok {
		t.Fatalf("expected deduct fail")
	}
}

func TestMove(t *testing.T) {
	b := New()
	b.Add(constants.BUCKET_AVAILABLE, 1000)
	if ok := b.Move(constants.BUCKET_AVAILABLE, constants.BUCKET_LOCKED, 400); !ok {
		t.Fatalf("expected move ok")
	}
	if b.Get(constants.BUCKET_AVAILABLE) != 600 {
		t.Fatalf("expected available 600")
	}
	if b.Get(constants.BUCKET_LOCKED) != 400 {
		t.Fatalf("expected locked 400")
	}
	if ok := b.Move(constants.BUCKET_AVAILABLE, constants.BUCKET_LOCKED, 700); ok {
		t.Fatalf("expected move fail")
	}
}

func TestSnapshotRestore(t *testing.T) {
	b := New()
	b.Add(constants.BUCKET_AVAILABLE, 100)
	b.Add(constants.BUCKET_LOCKED, 200)
	snap := b.Snapshot()

	b2 := New()
	b2.Restore(snap)
	if b2.Get(constants.BUCKET_AVAILABLE) != 100 {
		t.Fatalf("expected available 100")
	}
	if b2.Get(constants.BUCKET_LOCKED) != 200 {
		t.Fatalf("expected locked 200")
	}
}
