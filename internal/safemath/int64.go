package safemath

import (
	"math"
	"math/big"
	"sync"
)

var (
	maxInt64 = big.NewInt(math.MaxInt64)
	minInt64 = big.NewInt(math.MinInt64)
	intPool  sync.Pool
)

func Add(a, b int64) int64 {
	x := getInt()
	y := getInt()
	out := getInt()
	x.SetInt64(a)
	y.SetInt64(b)
	out.Add(x, y)
	res := clamp(out)
	putInt(x)
	putInt(y)
	putInt(out)
	return res
}

func Sub(a, b int64) int64 {
	x := getInt()
	y := getInt()
	out := getInt()
	x.SetInt64(a)
	y.SetInt64(b)
	out.Sub(x, y)
	res := clamp(out)
	putInt(x)
	putInt(y)
	putInt(out)
	return res
}

func Mul(a, b int64) int64 {
	if a == 0 || b == 0 {
		return 0
	}
	x := getInt()
	y := getInt()
	out := getInt()
	x.SetInt64(a)
	y.SetInt64(b)
	out.Mul(x, y)
	res := clamp(out)
	putInt(x)
	putInt(y)
	putInt(out)
	return res
}

func Div(a, b int64) int64 {
	if b == 0 || a == 0 {
		return 0
	}
	x := getInt()
	y := getInt()
	out := getInt()
	x.SetInt64(a)
	y.SetInt64(b)
	out.Quo(x, y)
	res := clamp(out)
	putInt(x)
	putInt(y)
	putInt(out)
	return res
}

func MulDiv(a, b, c int64) int64 {
	if c == 0 || a == 0 || b == 0 {
		return 0
	}
	x := getInt()
	y := getInt()
	z := getInt()
	out := getInt()
	x.SetInt64(a)
	y.SetInt64(b)
	z.SetInt64(c)
	out.Mul(x, y)
	out.Quo(out, z)
	res := clamp(out)
	putInt(x)
	putInt(y)
	putInt(z)
	putInt(out)
	return res
}

func WeightedAverage(priceA, qtyA, priceB, qtyB int64) int64 {
	totalQty := qtyA + qtyB
	if totalQty == 0 {
		return 0
	}
	pa := getInt()
	qa := getInt()
	pb := getInt()
	qb := getInt()
	sum := getInt()
	out := getInt()
	pa.SetInt64(priceA)
	qa.SetInt64(qtyA)
	pb.SetInt64(priceB)
	qb.SetInt64(qtyB)
	sum.Mul(pa, qa)
	out.Mul(pb, qb)
	sum.Add(sum, out)
	out.SetInt64(totalQty)
	sum.Quo(sum, out)
	res := clamp(sum)
	putInt(pa)
	putInt(qa)
	putInt(pb)
	putInt(qb)
	putInt(sum)
	putInt(out)
	return res
}

func clamp(v *big.Int) int64 {
	if v.Cmp(maxInt64) > 0 {
		return math.MaxInt64
	}
	if v.Cmp(minInt64) < 0 {
		return math.MinInt64
	}
	return v.Int64()
}

func getInt() *big.Int {
	if v := intPool.Get(); v != nil {
		return v.(*big.Int)
	}
	return new(big.Int)
}

func putInt(v *big.Int) {
	v.SetInt64(0)
	intPool.Put(v)
}
