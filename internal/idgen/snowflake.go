package idgen

import (
	"sync/atomic"
	"time"
)

const (
	nodeBits   = 10
	seqBits    = 12
	maxNode    = (1 << nodeBits) - 1
	maxSeq     = (1 << seqBits) - 1
	epochMilli = 1704067200000 // 2024-01-01T00:00:00Z
)

type Snowflake struct {
	node  uint64
	state uint64 // high bits: last ms, low bits: seq
}

func NewSnowflake(node uint16) *Snowflake {
	n := min(uint64(node), maxNode)
	return &Snowflake{node: n}
}

func (s *Snowflake) Next() uint64 {
	for {
		now := nowMillis()
		state := atomic.LoadUint64(&s.state)
		last := state >> seqBits
		seq := state & maxSeq

		if now < last {
			now = last
		}

		if now == last {
			if seq < maxSeq {
				seq++
			} else {
				for now <= last {
					now = nowMillis()
				}
				seq = 0
			}
		} else {
			seq = 0
		}

		newState := (now << seqBits) | seq
		if atomic.CompareAndSwapUint64(&s.state, state, newState) {
			return ((now - epochMilli) << (nodeBits + seqBits)) | (s.node << seqBits) | seq
		}
	}
}

func (s *Snowflake) State() (uint64, uint16) {
	state := atomic.LoadUint64(&s.state)
	last := state >> seqBits
	seq := state & maxSeq
	return last, uint16(seq)
}

func (s *Snowflake) Restore(lastMillis uint64, seq uint16) {
	state := (lastMillis << seqBits) | uint64(seq)
	atomic.StoreUint64(&s.state, state)
}

func (s *Snowflake) AdvanceTo(id uint64) {
	last := (id >> (nodeBits + seqBits)) + epochMilli
	idSeq := id & maxSeq
	for {
		state := atomic.LoadUint64(&s.state)
		curLast := state >> seqBits
		curSeq := state & maxSeq
		if last < curLast || (last == curLast && idSeq <= curSeq) {
			return
		}
		newState := (last << seqBits) | idSeq
		if atomic.CompareAndSwapUint64(&s.state, state, newState) {
			return
		}
	}
}

func ExtractTimestamp(id uint64) uint64 {
	return (id >> (nodeBits + seqBits)) + epochMilli
}

func nowMillis() uint64 {
	return uint64(time.Now().UnixNano() / int64(time.Millisecond))
}
