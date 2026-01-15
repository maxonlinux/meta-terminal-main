package math

import (
	"testing"

	"github.com/robaho/fixed"
)

func TestMul(t *testing.T) {
	a := fixed.NewI(10, 0)
	b := fixed.NewI(20, 0)
	result := Mul(a, b)
	expected := fixed.NewI(200, 0)
	if result.Cmp(expected) != 0 {
		t.Errorf("Mul(10, 20) = %d, want %d", result, expected)
	}
}

func TestDiv(t *testing.T) {
	a := fixed.NewI(100, 0)
	b := fixed.NewI(10, 0)
	result := Div(a, b)
	expected := fixed.NewI(10, 0)
	if result.Cmp(expected) != 0 {
		t.Errorf("Div(100, 10) = %d, want %d", result, expected)
	}
}

func TestMulDiv(t *testing.T) {
	a := fixed.NewI(10, 0)
	b := fixed.NewI(20, 0)
	c := fixed.NewI(5, 0)
	result := MulDiv(a, b, c)
	expected := fixed.NewI(40, 0)
	if result.Cmp(expected) != 0 {
		t.Errorf("MulDiv(10, 20, 5) = %d, want %d", result, expected)
	}
}

func TestAbs(t *testing.T) {
	tests := []struct {
		input    int64
		expected int64
	}{
		{10, 10},
		{-10, 10},
		{0, 0},
		{-1, 1},
	}
	for _, tt := range tests {
		if got := Abs(tt.input); got != tt.expected {
			t.Errorf("Abs(%d) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}

func TestMax(t *testing.T) {
	a := fixed.NewI(10, 0)
	b := fixed.NewI(20, 0)
	if got := Max(a, b); got.Cmp(b) != 0 {
		t.Errorf("Max(10, 20) = %d, want 20", got)
	}
}

func TestMin(t *testing.T) {
	a := fixed.NewI(10, 0)
	b := fixed.NewI(20, 0)
	if got := Min(a, b); got.Cmp(a) != 0 {
		t.Errorf("Min(10, 20) = %d, want 10", got)
	}
}

func TestNeg(t *testing.T) {
	n := fixed.NewI(10, 0)
	result := Neg(n)
	expected := fixed.NewI(-10, 0)
	if result.Cmp(expected) != 0 {
		t.Errorf("Neg(10) = %d, want %d", result, expected)
	}
}

func BenchmarkMul(b *testing.B) {
	a := fixed.NewI(1000000, 0)
	bb := fixed.NewI(50000, 0)
	for i := 0; i < b.N; i++ {
		Mul(a, bb)
	}
}

func BenchmarkMulDiv(b *testing.B) {
	a := fixed.NewI(1000000, 0)
	bb := fixed.NewI(50000, 0)
	c := fixed.NewI(10, 0)
	for i := 0; i < b.N; i++ {
		MulDiv(a, bb, c)
	}
}
