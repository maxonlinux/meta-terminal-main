package utils

import (
	"github.com/anomalyco/meta-terminal-go/internal/memory"
)

var (
	Scale = 8
)

func AbsInt64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

func Mul(price int64, qty int64) int64 {
	a := memory.GetBigInt()
	b := memory.GetBigInt()
	defer memory.PutBigInt(a)
	defer memory.PutBigInt(b)

	a.SetInt64(price)
	b.SetInt64(qty)
	a.Mul(a, b)
	return a.Int64()
}

func Div(numerator int64, denominator int64) int64 {
	if denominator == 0 {
		return 0
	}
	a := memory.GetBigInt()
	b := memory.GetBigInt()
	defer memory.PutBigInt(a)
	defer memory.PutBigInt(b)

	a.SetInt64(numerator)
	b.SetInt64(denominator)
	a.Quo(a, b)
	return a.Int64()
}

func Avg(oldPrice int64, oldQty int64, newPrice int64, newQty int64) int64 {
	a := memory.GetBigInt()
	b := memory.GetBigInt()
	c := memory.GetBigInt()
	d := memory.GetBigInt()
	defer memory.PutBigInt(a)
	defer memory.PutBigInt(b)
	defer memory.PutBigInt(c)
	defer memory.PutBigInt(d)

	a.SetInt64(oldPrice)
	b.SetInt64(oldQty)
	c.SetInt64(newPrice)
	d.SetInt64(newQty)

	a.Mul(a, b)
	c.Mul(c, d)
	b.Add(a, c)
	d.Add(b, d)
	a.Quo(b, d)
	return a.Int64()
}

func MulDiv(price int64, qty int64, divisor int64) int64 {
	if divisor == 0 {
		return 0
	}
	a := memory.GetBigInt()
	b := memory.GetBigInt()
	c := memory.GetBigInt()
	defer memory.PutBigInt(a)
	defer memory.PutBigInt(b)
	defer memory.PutBigInt(c)

	a.SetInt64(price)
	b.SetInt64(qty)
	c.SetInt64(divisor)

	a.Mul(a, b)
	a.Quo(a, c)
	return a.Int64()
}

func Sub(a int64, b int64) int64 {
	ia := memory.GetBigInt()
	ib := memory.GetBigInt()
	defer memory.PutBigInt(ia)
	defer memory.PutBigInt(ib)

	ia.SetInt64(a)
	ib.SetInt64(b)
	ia.Sub(ia, ib)
	return ia.Int64()
}

func Add(a int64, b int64) int64 {
	ia := memory.GetBigInt()
	ib := memory.GetBigInt()
	defer memory.PutBigInt(ia)
	defer memory.PutBigInt(ib)

	ia.SetInt64(a)
	ib.SetInt64(b)
	ia.Add(ia, ib)
	return ia.Int64()
}
