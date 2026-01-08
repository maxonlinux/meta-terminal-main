package snowflake

import (
	"sync"
	"time"
)

type Snowflake struct {
	mu        sync.Mutex
	timestamp int64
	machineID int64
	sequence  int64
}

var instance *Snowflake
var once sync.Once

func NextID() int64 {
	once.Do(func() {
		instance = &Snowflake{
			machineID: 1,
		}
	})
	return instance.generate()
}

func (s *Snowflake) generate() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UnixMilli()

	if now == s.timestamp {
		s.sequence++
		if s.sequence > 4095 {
			for now == s.timestamp {
				now = time.Now().UnixMilli()
			}
		}
	} else {
		s.sequence = 0
		s.timestamp = now
	}

	return (now << 22) | (s.machineID << 12) | s.sequence
}
