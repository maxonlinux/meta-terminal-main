package snowflake

import "sync/atomic"

var counter int64

// Next returns a unique monotonic ID for persisted data.
// Atomic increment keeps the generator thread-safe and fast.
func Next() int64 {
	return atomic.AddInt64(&counter, 1)
}
