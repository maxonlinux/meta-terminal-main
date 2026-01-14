package math

import "math/big"

func Mul(a, b *big.Int) *big.Int {
	return new(big.Int).Mul(a, b)
}

func Div(a, b *big.Int) *big.Int {
	if b.Sign() == 0 {
		return new(big.Int)
	}
	return new(big.Int).Quo(a, b)
}

func MulDiv(a, b, c *big.Int) *big.Int {
	if c.Sign() == 0 {
		return new(big.Int)
	}
	return new(big.Int).Quo(new(big.Int).Mul(a, b), c)
}

func Abs(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

func Max(a, b *big.Int) *big.Int {
	if a.Cmp(b) >= 0 {
		return a
	}
	return b
}

func Min(a, b *big.Int) *big.Int {
	if a.Cmp(b) <= 0 {
		return a
	}
	return b
}

func Neg(n *big.Int) *big.Int {
	return new(big.Int).Neg(n)
}
