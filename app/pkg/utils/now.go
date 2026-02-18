package utils

import "time"

// NowNano returns the current time in nanoseconds.
// Used as a single timestamp source for the engine.
func NowNano() uint64 {
	return uint64(time.Now().UnixNano())
}
