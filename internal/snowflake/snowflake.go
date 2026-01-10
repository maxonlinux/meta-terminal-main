package snowflake

import (
	"sync/atomic"
)

var counter int64

func Next() int64 {
	return atomic.AddInt64(&counter, 1)
}
