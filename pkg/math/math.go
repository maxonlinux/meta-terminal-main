package math

import "github.com/robaho/fixed"

var Zero = fixed.NewI(0, 0)

func Mul(a, b fixed.Fixed) fixed.Fixed {
	return a.Mul(b)
}

func Div(a, b fixed.Fixed) fixed.Fixed {
	if b.Sign() <= 0 {
		return Zero
	}
	return a.Div(b)
}

func MulDiv(a, b, c fixed.Fixed) fixed.Fixed {
	if c.Sign() <= 0 {
		return Zero
	}
	return a.Mul(b).Div(c)
}

func Abs(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

func Max(a, b fixed.Fixed) fixed.Fixed {
	if a.Cmp(b) >= 0 {
		return a
	}
	return b
}

func Min(a, b fixed.Fixed) fixed.Fixed {
	if a.Cmp(b) <= 0 {
		return a
	}
	return b
}

func Neg(n fixed.Fixed) fixed.Fixed {
	return Zero.Sub(n)
}

func Cmp(a, b fixed.Fixed) int {
	return a.Cmp(b)
}

func Sign(a fixed.Fixed) int {
	return a.Sign()
}

func Gt(a, b fixed.Fixed) bool {
	return a.GreaterThan(b)
}

func Gte(a, b fixed.Fixed) bool {
	return a.GreaterThanOrEqual(b)
}

func Lt(a, b fixed.Fixed) bool {
	return a.LessThan(b)
}

func Lte(a, b fixed.Fixed) bool {
	return a.LessThanOrEqual(b)
}

func Add(a, b fixed.Fixed) fixed.Fixed {
	return a.Add(b)
}

func Sub(a, b fixed.Fixed) fixed.Fixed {
	return a.Sub(b)
}
