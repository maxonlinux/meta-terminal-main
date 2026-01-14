package math

import (
	"math/big"
	"testing"
)

func TestMul(t *testing.T) {
	a := big.NewInt(10)
	b := big.NewInt(20)
	result := Mul(a, b)
	expected := big.NewInt(200)
	if result.Cmp(expected) != 0 {
		t.Errorf("Mul(10, 20) = %d, want %d", result, expected)
	}
}

func TestDiv(t *testing.T) {
	a := big.NewInt(100)
	b := big.NewInt(10)
	result := Div(a, b)
	expected := big.NewInt(10)
	if result.Cmp(expected) != 0 {
		t.Errorf("Div(100, 10) = %d, want %d", result, expected)
	}
}

func TestMulDiv(t *testing.T) {
	// (10 * 20) / 5 = 40
	a := big.NewInt(10)
	b := big.NewInt(20)
	c := big.NewInt(5)
	result := MulDiv(a, b, c)
	expected := big.NewInt(40)
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
	a := big.NewInt(10)
	b := big.NewInt(20)
	if got := Max(a, b); got.Cmp(b) != 0 {
		t.Errorf("Max(10, 20) = %d, want 20", got)
	}
}

func TestMin(t *testing.T) {
	a := big.NewInt(10)
	b := big.NewInt(20)
	if got := Min(a, b); got.Cmp(a) != 0 {
		t.Errorf("Min(10, 20) = %d, want 10", got)
	}
}

func TestNeg(t *testing.T) {
	n := big.NewInt(10)
	result := Neg(n)
	expected := big.NewInt(-10)
	if result.Cmp(expected) != 0 {
		t.Errorf("Neg(10) = %d, want %d", result, expected)
	}
}

func BenchmarkMul(b *testing.B) {
	a := big.NewInt(1000000)
	bb := big.NewInt(50000)
	for i := 0; i < b.N; i++ {
		Mul(a, bb)
	}
}

func BenchmarkMulDiv(b *testing.B) {
	a := big.NewInt(1000000)
	bb := big.NewInt(50000)
	c := big.NewInt(10)
	for i := 0; i < b.N; i++ {
		MulDiv(a, bb, c)
	}
}
