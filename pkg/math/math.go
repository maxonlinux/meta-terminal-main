// Package math provides low-level math primitives using math/big for precision.
// No business logic - only basic operations like Mul, Div, MulDiv, Abs, Max, Min.
package math

import "math/big"

// Mul returns a * b
func Mul(a, b *big.Int) *big.Int {
	return new(big.Int).Mul(a, b)
}

// Div returns a / b (integer division, truncates toward zero)
func Div(a, b *big.Int) *big.Int {
	if b.Sign() == 0 {
		return new(big.Int)
	}
	return new(big.Int).Quo(a, b)
}

// MulDiv returns a * b / c (avoids intermediate overflow)
// Equivalent to (a * b) / c with higher precision
func MulDiv(a, b, c *big.Int) *big.Int {
	if c.Sign() == 0 {
		return new(big.Int)
	}
	// Use Quo to get exact division
	return new(big.Int).Quo(new(big.Int).Mul(a, b), c)
}

// Abs returns absolute value of n
func Abs(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

// Max returns the maximum of a and b
func Max(a, b *big.Int) *big.Int {
	if a.Cmp(b) >= 0 {
		return a
	}
	return b
}

// Min returns the minimum of a and b
func Min(a, b *big.Int) *big.Int {
	if a.Cmp(b) <= 0 {
		return a
	}
	return b
}

// Neg returns -n
func Neg(n *big.Int) *big.Int {
	return new(big.Int).Neg(n)
}
