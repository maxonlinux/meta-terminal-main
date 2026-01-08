package balance

import (
	"testing"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
)

func TestBalance_Basics(t *testing.T) {
	b := New()
	b.Add(constants.BUCKET_AVAILABLE, 100)

	if got := b.Get(constants.BUCKET_AVAILABLE); got != 100 {
		t.Fatalf("available=%d", got)
	}

	if ok := b.Deduct(constants.BUCKET_AVAILABLE, 150); ok {
		t.Fatal("expected insufficient funds")
	}
	if ok := b.Deduct(constants.BUCKET_AVAILABLE, 40); !ok {
		t.Fatal("expected deduct ok")
	}
	if got := b.Get(constants.BUCKET_AVAILABLE); got != 60 {
		t.Fatalf("available=%d", got)
	}

	if ok := b.Move(constants.BUCKET_AVAILABLE, constants.BUCKET_LOCKED, 10); !ok {
		t.Fatal("expected move ok")
	}
	if got := b.Get(constants.BUCKET_AVAILABLE); got != 50 {
		t.Fatalf("available=%d", got)
	}
	if got := b.Get(constants.BUCKET_LOCKED); got != 10 {
		t.Fatalf("locked=%d", got)
	}
}

func TestState_Get_IsStable(t *testing.T) {
	s := NewState()
	b1 := s.Get(1, "USDT")
	b2 := s.Get(1, "USDT")
	if b1 != b2 {
		t.Fatal("expected same pointer")
	}
}
