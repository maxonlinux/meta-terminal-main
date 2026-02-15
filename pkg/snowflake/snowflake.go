package snowflake

import (
	"fmt"
	"sync"
	"time"
)

const (
	// snowflakeEpoch is 2024-01-01T00:00:00Z in milliseconds.
	snowflakeEpoch int64 = 1704067200000
	sequenceMask   int64 = (1 << 12) - 1
)

var (
	mu          sync.Mutex
	initialized bool
	currentID   int64
	lastMs      int64
	sequence    int64
)

// Init configures the Snowflake node ID for the process.
// Node IDs must be within 0-1023.
func Init(nodeID int64) error {
	mu.Lock()
	defer mu.Unlock()
	if initialized {
		if currentID != nodeID {
			return fmt.Errorf("snowflake already initialized with node %d", currentID)
		}
		return nil
	}
	if nodeID < 0 || nodeID > 1023 {
		return fmt.Errorf("snowflake node id out of range: %d", nodeID)
	}
	currentID = nodeID
	initialized = true
	lastMs = 0
	sequence = 0
	return nil
}

// Next returns a unique Snowflake ID for persisted data.
func Next() int64 {
	mu.Lock()
	if !initialized {
		currentID = 0
		initialized = true
	}
	mu.Unlock()
	return nextID()
}

func nextID() int64 {
	mu.Lock()
	defer mu.Unlock()

	nowMs := time.Now().UnixMilli()
	if nowMs < snowflakeEpoch {
		nowMs = snowflakeEpoch
	}
	if nowMs < lastMs {
		for nowMs < lastMs {
			time.Sleep(time.Millisecond)
			nowMs = time.Now().UnixMilli()
		}
	}

	if nowMs == lastMs {
		sequence = (sequence + 1) & sequenceMask
		if sequence == 0 {
			for nowMs <= lastMs {
				time.Sleep(time.Millisecond)
				nowMs = time.Now().UnixMilli()
			}
			lastMs = nowMs
		}
	} else {
		sequence = 0
		lastMs = nowMs
	}

	return ((nowMs - snowflakeEpoch) << 22) | (currentID << 12) | sequence
}
