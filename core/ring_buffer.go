package core

import "sync/atomic"

// RingBuffer implements a lock-free single-producer single-consumer ring buffer.
// Used for the LMAX Disruptor pattern - events are published by API handlers
// and consumed by a single event loop goroutine.
//
// Power of 2 capacity ensures fast modulo operations using bit masking.
type RingBuffer[T any] struct {
	buffer []T
	mask   uint64
	write  uint64
	read   uint64
}

// NewRingBuffer creates a ring buffer with the specified capacity.
func NewRingBuffer[T any](capacity uint64) *RingBuffer[T] {
	if capacity&(capacity-1) != 0 {
		panic("RingBuffer capacity must be a power of 2")
	}
	return &RingBuffer[T]{
		buffer: make([]T, capacity),
		mask:   capacity - 1,
	}
}

func (rb *RingBuffer[T]) Next() uint64 {
	return atomic.AddUint64(&rb.write, 1) - 1
}

func (rb *RingBuffer[T]) Publish(seq uint64) {
	atomic.StoreUint64(&rb.read, seq+1)
}

func (rb *RingBuffer[T]) Get(seq uint64) *T {
	return &rb.buffer[seq&rb.mask]
}

func (rb *RingBuffer[T]) ReadAvailable() uint64 {
	read := atomic.LoadUint64(&rb.read)
	write := atomic.LoadUint64(&rb.write)
	if write >= read {
		return write - read
	}
	return 0
}

func (rb *RingBuffer[T]) IsEmpty() bool {
	return rb.ReadAvailable() == 0
}

func (rb *RingBuffer[T]) Capacity() uint64 {
	return rb.mask + 1
}
