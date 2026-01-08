package utils

import (
	"math"
	"math/big"
)

var (
	bigInt0 = big.NewInt(0)
	bigInt1 = big.NewInt(1)
	bigInt2 = big.NewInt(2)
)

func Mul(a, b int64) int64 {
	if a == 0 || b == 0 {
		return 0
	}
	if a == 1 {
		return b
	}
	if b == 1 {
		return a
	}
	aa := big.NewInt(a)
	bb := big.NewInt(b)
	rr := new(big.Int).Mul(aa, bb)
	if rr.IsInt64() {
		return rr.Int64()
	}
	if (a > 0 && b > 0) || (a < 0 && b < 0) {
		return math.MaxInt64
	}
	return math.MinInt64
}

func Div(a, b int64) int64 {
	if b == 0 {
		return 0
	}
	if a == 0 {
		return 0
	}
	if b == 1 {
		return a
	}
	if a == b {
		return 1
	}
	aa := big.NewInt(a)
	bb := big.NewInt(b)
	rr := new(big.Int).Div(aa, bb)
	if rr.IsInt64() {
		return rr.Int64()
	}
	return 0
}

func MulDiv(a, b, c int64) int64 {
	if c == 0 {
		return 0
	}
	if a == 0 || b == 0 {
		return 0
	}
	aa := big.NewInt(a)
	bb := big.NewInt(b)
	cc := big.NewInt(c)
	rr := new(big.Int).Mul(aa, bb)
	rr = rr.Div(rr, cc)
	if rr.IsInt64() {
		return rr.Int64()
	}
	return 0
}

func Avg(prices []int64) int64 {
	if len(prices) == 0 {
		return 0
	}
	sum := big.NewInt(0)
	for _, p := range prices {
		sum.Add(sum, big.NewInt(p))
	}
	avg := sum.Div(sum, big.NewInt(int64(len(prices))))
	if avg.IsInt64() {
		return avg.Int64()
	}
	return 0
}

func Add(a, b int64) int64 {
	aa := big.NewInt(a)
	bb := big.NewInt(b)
	rr := new(big.Int).Add(aa, bb)
	if rr.IsInt64() {
		return rr.Int64()
	}
	if a > 0 {
		return math.MaxInt64
	}
	return math.MinInt64
}

func Sub(a, b int64) int64 {
	aa := big.NewInt(a)
	bb := big.NewInt(b)
	rr := new(big.Int).Sub(aa, bb)
	if rr.IsInt64() {
		return rr.Int64()
	}
	if a > 0 {
		return math.MaxInt64
	}
	return math.MinInt64
}
