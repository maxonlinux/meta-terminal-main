package users

import "testing"

func TestState_GetStable(t *testing.T) {
	s := NewState()
	u1 := s.Get(1)
	u2 := s.Get(1)
	if u1 != u2 {
		t.Fatal("expected same pointer")
	}
	if u1.GetBalance("USDT") == nil {
		t.Fatal("balance nil")
	}
	if u1.GetPosition("BTCUSDT") == nil {
		t.Fatal("position nil")
	}
}
