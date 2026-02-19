package shared

import "time"

// UnixMilliFromNano converts a nanosecond timestamp to unix milliseconds.
// Returns 0 when input is 0.
func UnixMilliFromNano(ts uint64) uint64 {
	if ts == 0 {
		return 0
	}
	return uint64(time.Unix(0, int64(ts)).UnixMilli())
}
