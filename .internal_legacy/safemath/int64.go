package safemath

import (
	"math"
	"math/bits"
)

func AddSaturating(a, b int64) int64 {
	if b > 0 && a > math.MaxInt64-b {
		return math.MaxInt64
	}
	if b < 0 && a < math.MinInt64-b {
		return math.MinInt64
	}
	return a + b
}

func SubSaturating(a, b int64) int64 {
	if b > 0 && a < math.MinInt64+b {
		return math.MinInt64
	}
	if b < 0 && a > math.MaxInt64+b {
		return math.MaxInt64
	}
	return a - b
}

func MulSaturating(a, b int64) int64 {
	if a == 0 || b == 0 {
		return 0
	}
	if a == 1 {
		return b
	}
	if b == 1 {
		return a
	}

	neg := (a < 0) != (b < 0)
	aa := uint64(abs64(a))
	bb := uint64(abs64(b))
	hi, lo := bits.Mul64(aa, bb)
	if hi != 0 || lo > uint64(math.MaxInt64) {
		if neg {
			return math.MinInt64
		}
		return math.MaxInt64
	}

	res := int64(lo)
	if neg {
		return -res
	}
	return res
}

func Div(a, b int64) int64 {
	if b == 0 {
		return 0
	}
	if a == 0 {
		return 0
	}
	return a / b
}

func MulDivSaturating(a, b, c int64) int64 {
	if c == 0 {
		return 0
	}
	return Div(MulSaturating(a, b), c)
}

func abs64(x int64) int64 {
	if x < 0 {
		if x == math.MinInt64 {
			return math.MaxInt64
		}
		return -x
	}
	return x
}
