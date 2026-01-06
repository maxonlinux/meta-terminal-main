package utils

func AbsInt64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}
